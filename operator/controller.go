package operator

import (
	"context"
	"fmt"
	"net/url"
	"runtime"
	"time"

	"github.com/grafana/agent/pkg/prom/instance"
	"github.com/nlowe/grafana-agent-operator/config"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	monitoringclientv1 "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	commonconfig "github.com/prometheus/common/config"
	promcfg "github.com/prometheus/prometheus/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
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
	serviceMoniotrInformer cache.SharedIndexInformer
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
	writer := config.NewWriter(&instance.RemoteWriteConfig{
		Base: promcfg.RemoteWriteConfig{URL: &commonconfig.URL{URL: u}},
	})

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
		serviceMoniotrInformer: smi.Informer(),
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

	c.log.Info("Fetching existing configs")
	existing, err := c.manager.ListScrapeConfigs()
	if err != nil {
		return fmt.Errorf("failed to list existing configs: %w", err)
	}

	knownServiceMonitors := map[string]struct{}{}
	for _, sm := range existing {
		knownServiceMonitors[sm] = struct{}{}
	}

	c.log.Info("Warming up the cache")
	warmup, cancel := context.WithTimeout(ctx, 1*time.Minute)
	ok := func() bool {
		defer cancel()
		return cache.WaitForCacheSync(warmup.Done(), c.serviceMoniotrInformer.HasSynced)
	}()
	if !ok {
		return fmt.Errorf("one or more caches failed to sync: %w", warmup.Err())
	}

	for _, obj := range c.serviceMoniotrInformer.GetStore().List() {
		for _, cfg := range c.configWriter.ScrapeConfigsForServiceMonitor(obj.(*monitoringv1.ServiceMonitor)) {
			delete(knownServiceMonitors, cfg.Name)
		}
	}

	c.log.Infof("Cleaning up %d service monitors that were removed while the operator was down", len(knownServiceMonitors))
	for sm := range knownServiceMonitors {
		log := c.log.WithField("name", sm)
		log.Debug("Cleaning up removed ServiceMonitor")
		// TODO: Somehow do this via the work queue instead?
		if err := c.manager.DeleteScrapeConfig(&instance.Config{Name: sm}); err != nil {
			log.WithError(err).Errorf("Failed to cleanup stale ServiceMonitor, ignoring...")
		}
	}

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
	for c.reconcile() {
	}
}

func fieldsForServiceMonitor(s *monitoringv1.ServiceMonitor) logrus.Fields {
	return logrus.Fields{"namespace": s.Namespace, "name": s.Name}
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
