package config

import (
	"regexp"

	"github.com/grafana/agent/pkg/prom/instance"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/pkg/relabel"
)

var invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)

type Writer interface {
	ScrapeConfigsForServiceMonitor(sm *v1.ServiceMonitor) []*instance.Config
}

type writer struct {
	rwc *config.RemoteWriteConfig
}

func NewWriter(rwc *config.RemoteWriteConfig) *writer {
	return &writer{rwc: rwc}
}

func makeRelabelConfigs(rlcs []*v1.RelabelConfig) []*relabel.Config {
	var results []*relabel.Config

	for _, c := range rlcs {
		rlc := &relabel.Config{
			Replacement: c.Replacement,
			TargetLabel: c.TargetLabel,
			Separator:   c.Separator,
			Action:      relabel.Action(c.Action),
			Modulus:     c.Modulus,
		}

		if c.Regex != "" {
			rlc.Regex = relabel.MustNewRegexp(c.Regex)
		}

		for _, l := range c.SourceLabels {
			rlc.SourceLabels = append(rlc.SourceLabels, model.LabelName(l))
		}

		results = append(results, rlc)
	}

	return results
}

func effectiveNamespaceSelector(sm *v1.ServiceMonitor) []string {
	// TODO: Global ignore at operator?
	if sm.Spec.NamespaceSelector.Any {
		return []string{}
	} else if len(sm.Spec.NamespaceSelector.MatchNames) == 0 {
		return []string{sm.Namespace}
	}

	return sm.Spec.NamespaceSelector.MatchNames
}

func sdConfig(namespaces []string) *kubernetes.SDConfig {
	cfg := &kubernetes.SDConfig{
		Role: kubernetes.RoleEndpoint,
	}

	if len(namespaces) != 0 {
		cfg.NamespaceDiscovery = kubernetes.NamespaceDiscovery{
			Names: namespaces,
		}
	}

	// TODO: Global API Server config?
	// TODO: TLS Config from global API Server config?
	return cfg
}

func safeLabelName(name string) string {
	return invalidLabelCharRE.ReplaceAllString(name, "_")
}
