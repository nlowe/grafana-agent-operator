package operator

import (
	"context"
	"net/url"
	"time"

	"github.com/grafana/agent/pkg/prom/instance"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const allNamespaces = ""

type watcher struct {
	wg wait.Group

	ctx context.Context
	log logrus.FieldLogger

	store      cache.Store
	controller cache.Controller
}

func NewWatcher(ctx context.Context, cfg *rest.Config) (*watcher, error) {
	c, err := monitoringv1client.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	w := &watcher{
		ctx: ctx,
		log: logrus.WithField("prefix", "watcher"),
	}

	// TODO: Watch PodMonitors as well
	w.store, w.controller = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(o metav1.ListOptions) (runtime.Object, error) {
				return c.ServiceMonitors(allNamespaces).List(ctx, o)
			},
			WatchFunc: func(o metav1.ListOptions) (watch.Interface, error) {
				return c.ServiceMonitors(allNamespaces).Watch(ctx, o)
			},
		},
		&v1.ServiceMonitor{},
		// TODO: Configure relist interval
		1*time.Minute,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    w.onAdd,
			UpdateFunc: w.onUpdate,
			DeleteFunc: w.onDelete,
		},
	)

	return w, nil
}

func (w *watcher) Run() cache.Store {
	w.wg.Start(func() {
		w.controller.Run(w.ctx.Done())
	})

	return w.store
}

func (w *watcher) Close() error {
	w.wg.Wait()
	return nil
}

func (w *watcher) onAdd(obj interface{}) {
	s, ok := obj.(*v1.ServiceMonitor)
	if !ok {
		w.log.Warn("Watcher discovered an added object that wasn't a service monitor")
		return
	}

	w.log.WithFields(logrus.Fields{
		"namespace": s.Namespace,
		"name":      s.Name,
	}).Info("Discovered ServiceMonitor")

	w.sync(s)
}

func (w *watcher) onUpdate(old, new interface{}) {
	sOld, ok := old.(*v1.ServiceMonitor)
	if !ok {
		w.log.Warn("Watcher discovered an updated object that wasn't a service monitor")
		return
	}

	sNew, ok := new.(*v1.ServiceMonitor)
	if !ok {
		w.log.Warn("Watcher discovered an updated object that wasn't a service monitor")
		return
	}

	if sOld.ResourceVersion == sNew.ResourceVersion {
		w.log.WithFields(logrus.Fields{
			"namespace": sOld.Namespace,
			"name":      sOld.Name,
		}).Debug("Got an update, but resource version is the same. Skipping")
		return
	}

	w.log.WithFields(logrus.Fields{
		"namespace": sNew.Namespace,
		"name":      sNew.Name,
	}).Info("Service Monitor Updated")

	w.sync(sNew)
}

func (w *watcher) onDelete(obj interface{}) {
	s, ok := obj.(*v1.ServiceMonitor)
	if !ok {
		w.log.Warn("Watcher discovered a removed object that wasn't a service monitor")
		return
	}

	w.log.WithFields(logrus.Fields{
		"namespace": s.Namespace,
		"name":      s.Name,
	}).Info("ServiceMonitor removed")
}

func (w *watcher) sync(s *v1.ServiceMonitor) {
	log := w.log.WithFields(logrus.Fields{
		"namespace": s.Namespace,
		"name":      s.Name,
	})

	log.Debug("Syncing ServiceMonitor")

	u, _ := url.Parse(viper.GetString("remote-write-url"))
	cfgs := MakeInstanceForServiceMonitor(&config.RemoteWriteConfig{URL: &commonconfig.URL{
		URL: u,
	}}, s)

	endpoint := viper.GetString("agent-url")
	for _, cfg := range cfgs {
		raw, _ := instance.MarshalConfig(cfg, true)
		log.Tracef("Generate config: \n%s\n", string(raw))

		if endpoint != "" {
			log.WithField("endpoint", endpoint).Trace("Syncing with grafana-agent")
			// TODO: POST to grafana-agent
		}
	}
}
