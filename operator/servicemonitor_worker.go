package operator

import (
	"fmt"

	"github.com/nlowe/grafana-agent-operator/k8sutil"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	SuccessfullySynced = "Synced"
	FailedSync         = "FailedSync"

	MessageSuccessfullySynced = "Scrape Configuration '%s' synced with agent"
)

func (c *Controller) reconcile() bool {
	item, shutdown := c.work.Get()

	if shutdown {
		return false
	}

	// Wrap the error so we can defer c.work.Done(...)
	utilruntime.HandleError(func(obj interface{}) error {
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
			err = c.deleteCachedKey(target.key)
		} else {
			err = c.syncCachedKey(target.key)
		}

		if err != nil {
			c.work.AddRateLimited(obj)
			return fmt.Errorf("error syncing or deleting %s %s: %w", target.kind, target.key, err)
		}

		c.work.Forget(obj)
		log.Info("Sync Complete")
		return nil
	}(item))

	return true
}

func (c *Controller) syncCachedKey(key string) error {
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

func (c *Controller) deleteCachedKey(key string) error {
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
