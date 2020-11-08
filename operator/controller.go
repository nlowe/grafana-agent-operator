package operator

import (
	"context"
	"fmt"
	"net/url"
	"runtime"
	"time"

	"github.com/nlowe/grafana-agent-operator/config"
	"github.com/nlowe/grafana-agent-operator/k8sutil"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	monitoringclientv1 "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	commonconfig "github.com/prometheus/common/config"
	promcfg "github.com/prometheus/prometheus/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kubernetesruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/deprecated/scheme"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const controllerAgentName = "grafana-agent-operator"

const (
	SuccessfullySynced = "Synced"
	FailedSync         = "FailedSync"

	MessageSuccessfullySynced = "Scrape Configuration '%s' synced with agent"
)

type monitorTarget struct {
	kind   string
	key    string
	delete bool
}

type Controller struct {
	k          kubernetes.Interface
	monitoring versioned.Interface

	factory externalversions.SharedInformerFactory

	serviceMonitorLister   monitoringclientv1.ServiceMonitorLister
	serviceMonitorSynced   cache.InformerSynced
	removedServiceMonitors cache.Indexer

	work     workqueue.RateLimitingInterface
	events   record.EventBroadcaster
	recorder record.EventRecorder

	configWriter config.Writer
	manager      ConfigManager

	log logrus.FieldLogger
}

func NewControllerForConfig(cfg *rest.Config, manager ConfigManager) (*Controller, error) {
	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	monitoring, err := versioned.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(viper.GetString("remote-write-url"))
	if err != nil {
		return nil, err
	}

	// TODO: Configure relist interval
	factory := externalversions.NewSharedInformerFactory(monitoring, 1*time.Minute)
	smi := factory.Monitoring().V1().ServiceMonitors()
	writer := config.NewWriter(&promcfg.RemoteWriteConfig{URL: &commonconfig.URL{URL: u}})

	log := logrus.WithField("prefix", "controller")

	events := record.NewBroadcaster()
	events.StartLogging(func(format string, args ...interface{}) {
		logrus.WithField("prefix", "controller/event").Tracef(format, args...)
	})
	events.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: k8s.CoreV1().Events("")})
	recorder := events.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	result := &Controller{
		k:          k8s,
		monitoring: monitoring,

		factory: factory,

		serviceMonitorLister:   smi.Lister(),
		serviceMonitorSynced:   smi.Informer().HasSynced,
		removedServiceMonitors: cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{}),

		work:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ServiceMonitors"),
		events:   events,
		recorder: recorder,

		manager:      manager,
		configWriter: writer,

		log: log,
	}

	// TODO: PodMonitors as well
	smi.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: result.enqueue,
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSmi := oldObj.(*monitoringv1.ServiceMonitor)
			newSmi := newObj.(*monitoringv1.ServiceMonitor)
			if oldSmi.ResourceVersion == newSmi.ResourceVersion {
				log.WithFields(fieldsForServiceMonitor(newSmi)).Debug("Ignoring already-synced ServiceMonitor")
				return
			}

			result.enqueue(newObj)
		},
		DeleteFunc: result.enqueueDelete,
	})

	return result, nil
}

func (c *Controller) Run(ctx context.Context) error {
	defer utilruntime.HandleCrash()
	defer c.events.Shutdown()
	defer c.work.ShutDown()

	c.log.Info("Starting Controller")
	go c.factory.Start(ctx.Done())

	c.log.Info("Warming up the cache")
	warmup, cancel := context.WithTimeout(ctx, 1*time.Minute)
	ok := func() bool {
		defer cancel()
		return cache.WaitForCacheSync(warmup.Done(), c.serviceMonitorSynced)
	}()
	if !ok {
		return fmt.Errorf("one or more caches failed to sync: %w", warmup.Err())
	}

	// TODO: Remove configs for ServiceMonitors that were removed while the operator was down

	c.log.Info("Starting Workers")
	// TODO: How many workers to we want to start?
	for i := 0; i < runtime.NumCPU(); i++ {
		go wait.Until(c.runWorker, time.Second, ctx.Done())
	}

	<-ctx.Done()
	c.log.Info("Shutting Down")
	return nil
}

func (c *Controller) runWorker() {
	for c.doWork() {
	}
}

func (c *Controller) doWork() bool {
	item, shutdown := c.work.Get()

	if shutdown {
		return false
	}

	// Wrap the error so we can defer c.work.Done(...)
	err := func(obj interface{}) error {
		defer c.work.Done(obj)

		var target monitorTarget
		var ok bool

		if target, ok = obj.(monitorTarget); !ok {
			c.work.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected monitorTarget in work queue but got %#v", obj))
			return nil
		}

		log := c.log.WithField("serviceMonitor", target.key)

		var err error
		if target.delete {
			err = c.doDelete(target.key)
		} else {
			err = c.doSync(target.key)
		}

		if err != nil {
			c.work.AddRateLimited(obj)
			return fmt.Errorf("error syncing or deleting %s %s: %w", target.kind, target.key, err)
		}

		c.work.Forget(obj)
		log.Info("Sync Complete")
		return nil
	}(item)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) doSync(key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key '%s': %w", key, err))
		return nil
	}

	sm, err := c.serviceMonitorLister.ServiceMonitors(ns).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("ServiceMonitor '%s' no longer exists", key))
			return nil
		}

		return err
	}

	if err := k8sutil.AddTypeMetaToObject(sm); err != nil {
		return err
	}

	c.log.WithField("serviceMonitor", key).Debug("Creating or updating scrape configs")
	for _, cfg := range c.configWriter.ScrapeConfigsForServiceMonitor(sm) {
		if err := c.manager.UpdateScrapeConfig(cfg); err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to sync config: %w", err))
			c.recorder.Event(sm, corev1.EventTypeWarning, FailedSync, err.Error())
			return err
		}

		c.recorder.Event(
			sm,
			corev1.EventTypeNormal,
			SuccessfullySynced,
			fmt.Sprintf(MessageSuccessfullySynced, cfg.Name),
		)
	}

	return nil
}

func (c *Controller) doDelete(key string) error {
	obj, exists, err := c.removedServiceMonitors.GetByKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to get removed service monitor from cache: %w", err))
		return err
	}

	var sm *monitoringv1.ServiceMonitor

	if !exists {
		ns, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("invalid resource key '%s': %w", key, err))
			return nil
		}

		sm, err = c.serviceMonitorLister.ServiceMonitors(ns).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				utilruntime.HandleError(fmt.Errorf("ServiceMonitor '%s' no longer exists", key))
				return nil
			}

			return err
		}
	} else {
		sm = obj.(*monitoringv1.ServiceMonitor)
	}

	c.log.WithField("serviceMonitor", key).Debug("Calculating scrape configs to delete")
	for _, cfg := range c.configWriter.ScrapeConfigsForServiceMonitor(sm) {
		if err := c.manager.DeleteScrapeConfig(cfg); err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to sync config: %w", err))
			return err
		}
	}

	return nil
}

func (c *Controller) enqueue(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	if err := c.removedServiceMonitors.Delete(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.log.WithField("serviceMonitor", key).Trace("enqueuing sync")
	c.work.Add(monitorTarget{kind: obj.(kubernetesruntime.Object).GetObjectKind().GroupVersionKind().Kind, key: key})
}

func (c *Controller) enqueueDelete(obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	if err := c.removedServiceMonitors.Add(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.log.WithField("serviceMonitor", key).Trace("enqueuing delete")
	c.work.Add(monitorTarget{kind: obj.(kubernetesruntime.Object).GetObjectKind().GroupVersionKind().Kind, key: key, delete: true})
}

func fieldsForServiceMonitor(s *monitoringv1.ServiceMonitor) logrus.Fields {
	return logrus.Fields{"namespace": s.Namespace, "name": s.Name}
}
