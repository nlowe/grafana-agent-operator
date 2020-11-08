package config

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/grafana/agent/pkg/prom/instance"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/pkg/relabel"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (w *writer) ScrapeConfigsForServiceMonitor(sm *v1.ServiceMonitor) []*instance.Config {
	results := make([]*instance.Config, len(sm.Spec.Endpoints))

	for i, ep := range sm.Spec.Endpoints {
		results[i] = w.makeInstanceForServiceMonitorEndpoint(sm, ep, i)
	}

	return results
}

func (w *writer) makeInstanceForServiceMonitorEndpoint(sm *v1.ServiceMonitor, ep v1.Endpoint, endpointNumber int) *instance.Config {
	// TODO: Can we contribute to the operator to write this for us? This is mostly copied from the operator
	//       See https://github.com/prometheus-operator/prometheus-operator/blob/d97ba662bc94d64e254e116f3cbf573068ac2d87/pkg/prometheus/promcfg.go#L851
	honorTimestamps := false
	if ep.HonorTimestamps != nil {
		// TODO: Override at the operator level?
		honorTimestamps = *ep.HonorTimestamps
	}

	name := fmt.Sprintf("%s/%s/%d", sm.Namespace, sm.Name, endpointNumber)
	namespaces := effectiveNamespaceSelector(sm)

	// TODO: What should we do about errors when building up the config?
	sc := &config.ScrapeConfig{
		JobName: name,
		// TODO: Override at the operator level?
		HonorLabels:             ep.HonorLabels,
		HonorTimestamps:         honorTimestamps,
		ServiceDiscoveryConfigs: discovery.Configs{sdConfig(namespaces)},
		SampleLimit:             uint(sm.Spec.SampleLimit),
		TargetLimit:             uint(sm.Spec.TargetLimit),
	}

	if ep.Interval != "" {
		sc.ScrapeInterval, _ = model.ParseDuration(ep.Interval)
	}

	if ep.ScrapeTimeout != "" {
		sc.ScrapeTimeout, _ = model.ParseDuration(ep.ScrapeTimeout)
	}

	if ep.Path != "" {
		sc.MetricsPath = ep.Path
	}

	if ep.ProxyURL != nil {
		u, _ := url.Parse(*ep.ProxyURL)
		// TODO: Handle error
		sc.HTTPClientConfig.ProxyURL = commonconfig.URL{URL: u}
	}

	if ep.Params != nil {
		sc.Params = ep.Params
	}

	if ep.Scheme != "" {
		sc.Scheme = ep.Scheme
	}

	if ep.TLSConfig != nil {
		sc.HTTPClientConfig.TLSConfig.InsecureSkipVerify = ep.TLSConfig.InsecureSkipVerify
		//if ep.TLSConfig.CA.Secret != nil || ep.TLSConfig.CA.ConfigMap != nil {
		//	// TODO: Lookup from secret / configmap
		//}
		//
		//if ep.TLSConfig.Cert.Secret != nil || ep.TLSConfig.Cert.ConfigMap != nil {
		//	// TODO: Lookup from secret / configmap
		//}
		//
		//if ep.TLSConfig.KeySecret != nil {
		//	// TODO: Lookup from secret
		//}

		sc.HTTPClientConfig.TLSConfig.ServerName = ep.TLSConfig.ServerName
	}

	if ep.BearerTokenFile != "" {
		sc.HTTPClientConfig.BearerTokenFile = ep.BearerTokenFile
	}

	//if ep.BearerTokenSecret.Name != "" {
	//	// TODO: Bearer token secrets
	//}

	var labelKeys []string
	for k := range sm.Spec.Selector.MatchLabels {
		labelKeys = append(labelKeys, k)
	}
	sort.Strings(labelKeys)

	for _, k := range labelKeys {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
			Action:       relabel.Keep,
			SourceLabels: []model.LabelName{model.LabelName("__meta_kubernetes_service_label_" + safeLabelName(k))},
			Regex:        relabel.MustNewRegexp(sm.Spec.Selector.MatchLabels[k]),
		})
	}

	for _, exp := range sm.Spec.Selector.MatchExpressions {
		switch exp.Operator {
		case metav1.LabelSelectorOpIn:
			sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
				Action:       relabel.Keep,
				SourceLabels: []model.LabelName{model.LabelName("__meta_kubernetes_service_label_" + safeLabelName(exp.Key))},
				Regex:        relabel.MustNewRegexp(strings.Join(exp.Values, "|")),
			})
		case metav1.LabelSelectorOpNotIn:
			sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
				Action:       relabel.Drop,
				SourceLabels: []model.LabelName{model.LabelName("__meta_kubernetes_service_label_" + safeLabelName(exp.Key))},
				Regex:        relabel.MustNewRegexp(strings.Join(exp.Values, "|")),
			})
		case metav1.LabelSelectorOpExists:
			sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
				Action:       relabel.Keep,
				SourceLabels: []model.LabelName{model.LabelName("__meta_kubernetes_service_label_" + safeLabelName(exp.Key))},
				Regex:        relabel.MustNewRegexp(".*"),
			})
		case metav1.LabelSelectorOpDoesNotExist:
			sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
				Action:       relabel.Drop,
				SourceLabels: []model.LabelName{model.LabelName("__meta_kubernetes_service_label_" + safeLabelName(exp.Key))},
				Regex:        relabel.MustNewRegexp(".*"),
			})
		}
	}

	if ep.Port != "" {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
			Action:       relabel.Keep,
			SourceLabels: []model.LabelName{"__meta_kubernetes_endpoint_port_name"},
			Regex:        relabel.MustNewRegexp(ep.Port),
		})
	} else if ep.TargetPort != nil {
		if ep.TargetPort.StrVal != "" {
			sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
				Action:       relabel.Keep,
				SourceLabels: []model.LabelName{"__meta_kubernetes_endpoint_port_name"},
				Regex:        relabel.MustNewRegexp(ep.TargetPort.String()),
			})
		} else if ep.TargetPort.IntVal != 0 {
			sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
				Action:       relabel.Keep,
				SourceLabels: []model.LabelName{"__meta_kubernetes_endpoint_port_number"},
				Regex:        relabel.MustNewRegexp(ep.TargetPort.String()),
			})
		}
	}

	sc.RelabelConfigs = append(sc.RelabelConfigs, []*relabel.Config{
		{
			SourceLabels: []model.LabelName{"__meta_kubernetes_endpoint_address_target_kind", "__meta_kubernetes_endpoint_address_target_name"},
			Separator:    ";",
			Regex:        relabel.MustNewRegexp("Node;(.*)"),
			Replacement:  "${1}",
			TargetLabel:  "node",
		},
		{
			SourceLabels: []model.LabelName{"__meta_kubernetes_endpoint_address_target_kind", "__meta_kubernetes_endpoint_address_target_name"},
			Separator:    ";",
			Regex:        relabel.MustNewRegexp("Pod;(.*)"),
			Replacement:  "${1}",
			TargetLabel:  "pod",
		},
		{
			SourceLabels: []model.LabelName{"__meta_kubernetes_namespace"},
			TargetLabel:  "namespace",
		},
		{
			SourceLabels: []model.LabelName{"__meta_kubernetes_service_name"},
			TargetLabel:  "service_name",
		},
		{
			SourceLabels: []model.LabelName{"__meta_kubernetes_pod_name"},
			TargetLabel:  "pod",
		},
		{
			SourceLabels: []model.LabelName{"__meta_kubernetes_pod_container_name"},
			TargetLabel:  "container",
		},
	}...)

	// Save labels from the service
	for _, l := range sm.Spec.TargetLabels {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
			SourceLabels: []model.LabelName{model.LabelName("__meta_kubernetes_service_label_" + safeLabelName(l))},
			TargetLabel:  safeLabelName(l),
			Regex:        relabel.MustNewRegexp("(.+)"),
			Replacement:  "${1}",
		})
	}

	// Save labels from the discovered pods
	for _, l := range sm.Spec.PodTargetLabels {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
			SourceLabels: []model.LabelName{model.LabelName("__meta_kubernetes_pod_label_" + safeLabelName(l))},
			TargetLabel:  safeLabelName(l),
			Regex:        relabel.MustNewRegexp("(.+)"),
			Replacement:  "${1}",
		})
	}

	// Default the job label to the service name
	sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
		SourceLabels: []model.LabelName{"__meta_kubernetes_service_name"},
		TargetLabel:  "job",
		Replacement:  "${1}",
	})

	// Add a relabel to pick the job name from the specified label if it exists
	if sm.Spec.JobLabel != "" {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
			SourceLabels: []model.LabelName{model.LabelName("__meta_kubernetes_service_label_" + safeLabelName(sm.Spec.JobLabel))},
			TargetLabel:  "job",
			Regex:        relabel.MustNewRegexp("(.+)"),
			Replacement:  "${1}",
		})
	}

	if ep.Port != "" {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
			TargetLabel: "endpoint",
			Replacement: ep.Port,
		})
	} else if ep.TargetPort != nil && ep.TargetPort.String() != "" {
		sc.RelabelConfigs = append(sc.RelabelConfigs, &relabel.Config{
			TargetLabel: "endpoint",
			Replacement: ep.TargetPort.String(),
		})
	}

	sc.RelabelConfigs = append(sc.RelabelConfigs, makeRelabelConfigs(ep.RelabelConfigs)...)

	// TODO: Enforce Namespace Label from the operator?

	sc.RelabelConfigs = append(sc.RelabelConfigs, makeRelabelConfigs(ep.MetricRelabelConfigs)...)

	return &instance.Config{
		Name:          name,
		ScrapeConfigs: []*config.ScrapeConfig{sc},
		RemoteWrite:   []*config.RemoteWriteConfig{w.rwc},
	}
}
