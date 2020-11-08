package config

import (
	"net/url"
	"strings"
	"testing"

	"github.com/grafana/agent/pkg/prom/instance"
	v1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/pkg/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func genConfig(sut *writer, ep v1.Endpoint) *instance.Config {
	return sut.makeInstanceForServiceMonitorEndpoint(&v1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dummy",
			Namespace: "myapp",
		},
		Spec: v1.ServiceMonitorSpec{
			Endpoints: []v1.Endpoint{ep},
		},
	}, ep, 0)
}

func getSDConfig(i *instance.Config) *kubernetes.SDConfig {
	return i.ScrapeConfigs[0].ServiceDiscoveryConfigs[0].(*kubernetes.SDConfig)
}

func TestMakeInstanceForServiceMonitor(t *testing.T) {
	u, _ := url.Parse("http://cortex.monitoring.svc.cluster.local/api/prom/push")
	sut := &writer{rwc: &config.RemoteWriteConfig{URL: &commonconfig.URL{URL: u}}}

	t.Run("Instance Per Endpoint", func(t *testing.T) {
		configs := sut.ScrapeConfigsForServiceMonitor(&v1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dummy",
				Namespace: "myapp",
			},
			Spec: v1.ServiceMonitorSpec{
				Endpoints: []v1.Endpoint{
					{Port: "a"},
					{Port: "b"},
					{Port: "c"},
				},
			},
		})

		require.Len(t, configs, 3)
	})

	t.Run("Config Generation", func(t *testing.T) {
		t.Run("Named Properly", func(t *testing.T) {
			cfg := genConfig(sut, v1.Endpoint{})

			require.Equal(t, "myapp/dummy/0", cfg.Name)
		})

		t.Run("Sets RemoteWriteConfig", func(t *testing.T) {
			cfg := genConfig(sut, v1.Endpoint{})

			require.Len(t, cfg.RemoteWrite, 1)
			assert.Equal(t, u.String(), cfg.RemoteWrite[0].URL.String())
		})

		t.Run("Honor Timestamps", func(t *testing.T) {
			t.Run("False", func(t *testing.T) {
				v := false
				cfg := genConfig(sut, v1.Endpoint{HonorTimestamps: &v})
				require.False(t, cfg.ScrapeConfigs[0].HonorTimestamps)
			})

			t.Run("True", func(t *testing.T) {
				v := true
				cfg := genConfig(sut, v1.Endpoint{HonorTimestamps: &v})
				require.True(t, cfg.ScrapeConfigs[0].HonorTimestamps)
			})
		})

		t.Run("Honor Labels", func(t *testing.T) {
			t.Run("False", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{HonorLabels: false})
				require.False(t, cfg.ScrapeConfigs[0].HonorLabels)
			})

			t.Run("True", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{HonorLabels: true})
				require.True(t, cfg.ScrapeConfigs[0].HonorLabels)
			})
		})

		t.Run("Kubernetes SD Configs", func(t *testing.T) {
			t.Run("Endpoint Mode", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{})
				require.Equal(t, kubernetes.RoleEndpoint, getSDConfig(cfg).Role)
			})

			t.Run("Namespace Selector Any", func(t *testing.T) {
				cfg := sut.makeInstanceForServiceMonitorEndpoint(&v1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: "myapp",
					},
					Spec: v1.ServiceMonitorSpec{
						Endpoints:         []v1.Endpoint{},
						NamespaceSelector: v1.NamespaceSelector{Any: true},
					},
				}, v1.Endpoint{}, 0)

				require.Empty(t, getSDConfig(cfg).NamespaceDiscovery.Names)
			})

			t.Run("Same Namespace", func(t *testing.T) {
				cfg := sut.makeInstanceForServiceMonitorEndpoint(&v1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: "myapp",
					},
					Spec: v1.ServiceMonitorSpec{
						Endpoints: []v1.Endpoint{},
					},
				}, v1.Endpoint{}, 0)

				result := getSDConfig(cfg).NamespaceDiscovery.Names
				assert.Len(t, result, 1)
				assert.Equal(t, "myapp", result[0])
			})

			t.Run("Match Names", func(t *testing.T) {
				cfg := sut.makeInstanceForServiceMonitorEndpoint(&v1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: "myapp",
					},
					Spec: v1.ServiceMonitorSpec{
						Endpoints:         []v1.Endpoint{},
						NamespaceSelector: v1.NamespaceSelector{MatchNames: []string{"a", "b"}},
					},
				}, v1.Endpoint{}, 0)

				result := getSDConfig(cfg).NamespaceDiscovery.Names
				assert.Len(t, result, 2)
				assert.Contains(t, result, "a")
				assert.Contains(t, result, "b")
			})
		})

		t.Run("Sample Limit", func(t *testing.T) {
			t.Run("Not Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{})
				require.Zero(t, cfg.ScrapeConfigs[0].ScrapeInterval)
			})

			t.Run("Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{Interval: "30s"})
				require.Equal(t, "30s", cfg.ScrapeConfigs[0].ScrapeInterval.String())
			})
		})

		t.Run("Timeout", func(t *testing.T) {
			t.Run("Not Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{})
				require.Zero(t, cfg.ScrapeConfigs[0].ScrapeTimeout)
			})

			t.Run("Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{ScrapeTimeout: "30s"})
				require.Equal(t, "30s", cfg.ScrapeConfigs[0].ScrapeTimeout.String())
			})
		})

		t.Run("Path", func(t *testing.T) {
			t.Run("Not Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{})
				require.Zero(t, cfg.ScrapeConfigs[0].MetricsPath)
			})

			t.Run("Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{Path: "/foo/bar"})
				require.Equal(t, "/foo/bar", cfg.ScrapeConfigs[0].MetricsPath)
			})
		})

		t.Run("ProxyURL", func(t *testing.T) {
			t.Run("Not Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{})
				require.Zero(t, cfg.ScrapeConfigs[0].HTTPClientConfig.ProxyURL)
			})

			t.Run("Set", func(t *testing.T) {
				v := "http://proxy:9999/foo"
				cfg := genConfig(sut, v1.Endpoint{ProxyURL: &v})
				require.Equal(t, v, cfg.ScrapeConfigs[0].HTTPClientConfig.ProxyURL.String())
			})
		})

		t.Run("Params", func(t *testing.T) {
			t.Run("Not Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{})
				require.Zero(t, cfg.ScrapeConfigs[0].Params)
			})

			t.Run("Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{Params: map[string][]string{"foo": {"bar"}}})
				require.Equal(t, "bar", cfg.ScrapeConfigs[0].Params.Get("foo"))
			})
		})

		t.Run("Scheme", func(t *testing.T) {
			t.Run("Not Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{})
				require.Zero(t, cfg.ScrapeConfigs[0].Scheme)
			})

			t.Run("Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{Scheme: "https"})
				require.Equal(t, "https", cfg.ScrapeConfigs[0].Scheme)
			})
		})

		t.Run("TLS Config", func(t *testing.T) {
			t.Run("Insecure Skip Verify", func(t *testing.T) {
				t.Run("False", func(t *testing.T) {
					cfg := genConfig(sut, v1.Endpoint{TLSConfig: &v1.TLSConfig{SafeTLSConfig: v1.SafeTLSConfig{InsecureSkipVerify: false}}})
					require.False(t, cfg.ScrapeConfigs[0].HTTPClientConfig.TLSConfig.InsecureSkipVerify)
				})

				t.Run("Set", func(t *testing.T) {
					cfg := genConfig(sut, v1.Endpoint{TLSConfig: &v1.TLSConfig{SafeTLSConfig: v1.SafeTLSConfig{InsecureSkipVerify: true}}})
					require.True(t, cfg.ScrapeConfigs[0].HTTPClientConfig.TLSConfig.InsecureSkipVerify)
				})
			})

			t.Run("Server Name", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{TLSConfig: &v1.TLSConfig{SafeTLSConfig: v1.SafeTLSConfig{ServerName: "foo.bar"}}})
				require.Equal(t, "foo.bar", cfg.ScrapeConfigs[0].HTTPClientConfig.TLSConfig.ServerName)
			})
		})

		t.Run("Bearer Token File", func(t *testing.T) {
			t.Run("Not Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{})
				require.Zero(t, cfg.ScrapeConfigs[0].Scheme)
			})

			t.Run("Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{BearerTokenFile: "foo/bar.token"})
				require.Equal(t, "foo/bar.token", cfg.ScrapeConfigs[0].HTTPClientConfig.BearerTokenFile)
			})
		})

		t.Run("Match Labels", func(t *testing.T) {
			cfg := sut.makeInstanceForServiceMonitorEndpoint(&v1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy",
					Namespace: "myapp",
				},
				Spec: v1.ServiceMonitorSpec{
					Endpoints: []v1.Endpoint{},
					Selector: metav1.LabelSelector{MatchLabels: map[string]string{
						"z":     "1",
						"b":     "2",
						"a/b/c": "3",
					}},
				},
			}, v1.Endpoint{}, 0)

			assertRLC(t, cfg.ScrapeConfigs[0], relabel.Keep, "__meta_kubernetes_service_label_a_b_c", "^(?:3)$")
			assertRLC(t, cfg.ScrapeConfigs[0], relabel.Keep, "__meta_kubernetes_service_label_b", "^(?:2)$")
			assertRLC(t, cfg.ScrapeConfigs[0], relabel.Keep, "__meta_kubernetes_service_label_z", "^(?:1)$")

			t.Run("Sorted", func(t *testing.T) {
				assert.Equal(t, "__meta_kubernetes_service_label_a_b_c", string(cfg.ScrapeConfigs[0].RelabelConfigs[0].SourceLabels[0]))
				assert.Equal(t, "__meta_kubernetes_service_label_b", string(cfg.ScrapeConfigs[0].RelabelConfigs[1].SourceLabels[0]))
				assert.Equal(t, "__meta_kubernetes_service_label_z", string(cfg.ScrapeConfigs[0].RelabelConfigs[2].SourceLabels[0]))
			})
		})

		t.Run("Match Expressions", func(t *testing.T) {
			cfg := sut.makeInstanceForServiceMonitorEndpoint(&v1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy",
					Namespace: "myapp",
				},
				Spec: v1.ServiceMonitorSpec{
					Endpoints: []v1.Endpoint{},
					Selector: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
						{Operator: metav1.LabelSelectorOpIn, Key: "in", Values: []string{"a", "b"}},
						{Operator: metav1.LabelSelectorOpNotIn, Key: "notin", Values: []string{"c", "d"}},
						{Operator: metav1.LabelSelectorOpExists, Key: "exists"},
						{Operator: metav1.LabelSelectorOpDoesNotExist, Key: "notexists"},
					}},
				},
			}, v1.Endpoint{}, 0)

			assertRLC(t, cfg.ScrapeConfigs[0], relabel.Keep, "__meta_kubernetes_service_label_in", "^(?:a|b)$")
			assertRLC(t, cfg.ScrapeConfigs[0], relabel.Drop, "__meta_kubernetes_service_label_notin", "^(?:c|d)$")
			assertRLC(t, cfg.ScrapeConfigs[0], relabel.Keep, "__meta_kubernetes_service_label_exists", "^(?:.*)$")
			assertRLC(t, cfg.ScrapeConfigs[0], relabel.Drop, "__meta_kubernetes_service_label_notexists", "^(?:.*)$")
		})

		t.Run("Port", func(t *testing.T) {
			t.Run("Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{Port: "metrics"})

				assertRLC(t, cfg.ScrapeConfigs[0], relabel.Keep, "__meta_kubernetes_endpoint_port_name", "^(?:metrics)$")
			})

			t.Run("Target Port Set", func(t *testing.T) {
				t.Run("String", func(t *testing.T) {
					v := intstr.FromString("metrics")
					cfg := genConfig(sut, v1.Endpoint{TargetPort: &v})

					assertRLC(t, cfg.ScrapeConfigs[0], relabel.Keep, "__meta_kubernetes_endpoint_port_name", "^(?:metrics)$")
				})

				t.Run("Int", func(t *testing.T) {
					v := intstr.FromInt(9000)
					cfg := genConfig(sut, v1.Endpoint{TargetPort: &v})

					assertRLC(t, cfg.ScrapeConfigs[0], relabel.Keep, "__meta_kubernetes_endpoint_port_number", "^(?:9000)$")
				})
			})

			t.Run("Not Set", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{})
				for _, rlc := range cfg.ScrapeConfigs[0].RelabelConfigs {
					for _, l := range rlc.SourceLabels {
						if strings.HasPrefix(string(l), "__meta_kubernetes_endpoint_port_") {
							t.Errorf("Found unexpected RLC: %s", l)
						}
					}
				}
			})
		})

		t.Run("Constant RLCs", func(t *testing.T) {
			cfg := genConfig(sut, v1.Endpoint{})

			assertRLCWith(t, cfg.ScrapeConfigs[0], func(rlc *relabel.Config) bool {
				return len(rlc.SourceLabels) == 2 &&
					rlc.SourceLabels[0] == "__meta_kubernetes_endpoint_address_target_kind" &&
					rlc.SourceLabels[1] == "__meta_kubernetes_endpoint_address_target_name" &&
					rlc.Regex.String() == "^(?:Pod;(.*))$"
			}, func(t *testing.T, rlc *relabel.Config) {
				assert.Equal(t, ";", rlc.Separator)
				assert.Equal(t, "${1}", rlc.Replacement)
				assert.Equal(t, "pod", rlc.TargetLabel)
			})

			assertRLCWith(t, cfg.ScrapeConfigs[0], func(rlc *relabel.Config) bool {
				return len(rlc.SourceLabels) == 2 &&
					rlc.SourceLabels[0] == "__meta_kubernetes_endpoint_address_target_kind" &&
					rlc.SourceLabels[1] == "__meta_kubernetes_endpoint_address_target_name" &&
					rlc.Regex.String() == "^(?:Node;(.*))$"
			}, func(t *testing.T, rlc *relabel.Config) {
				assert.Equal(t, ";", rlc.Separator)
				assert.Equal(t, "${1}", rlc.Replacement)
				assert.Equal(t, "node", rlc.TargetLabel)
			})

			assertRLCTarget(t, cfg.ScrapeConfigs[0], "__meta_kubernetes_namespace", "namespace")
			assertRLCTarget(t, cfg.ScrapeConfigs[0], "__meta_kubernetes_service_name", "service_name")
			assertRLCTarget(t, cfg.ScrapeConfigs[0], "__meta_kubernetes_pod_name", "pod")
			assertRLCTarget(t, cfg.ScrapeConfigs[0], "__meta_kubernetes_pod_container_name", "container")
		})

		targetLabelTest := func(target string) func(t *testing.T, rlc *relabel.Config) {
			return func(t *testing.T, rlc *relabel.Config) {
				assert.Equal(t, target, rlc.TargetLabel)
				assert.Equal(t, "^(?:(.+))$", rlc.Regex.String())
				assert.Equal(t, "${1}", rlc.Replacement)
			}
		}

		t.Run("Target Labels", func(t *testing.T) {
			cfg := sut.makeInstanceForServiceMonitorEndpoint(&v1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy",
					Namespace: "myapp",
				},
				Spec: v1.ServiceMonitorSpec{
					Endpoints:    []v1.Endpoint{},
					TargetLabels: []string{"a", "b", "c/d/e"},
				},
			}, v1.Endpoint{}, 0)

			assertRLCWith(t, cfg.ScrapeConfigs[0], rlcMatchSingle("__meta_kubernetes_service_label_a"), targetLabelTest("a"))
			assertRLCWith(t, cfg.ScrapeConfigs[0], rlcMatchSingle("__meta_kubernetes_service_label_b"), targetLabelTest("b"))
			assertRLCWith(t, cfg.ScrapeConfigs[0], rlcMatchSingle("__meta_kubernetes_service_label_c_d_e"), targetLabelTest("c_d_e"))
		})

		t.Run("Pod Labels", func(t *testing.T) {
			cfg := sut.makeInstanceForServiceMonitorEndpoint(&v1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy",
					Namespace: "myapp",
				},
				Spec: v1.ServiceMonitorSpec{
					Endpoints:       []v1.Endpoint{},
					PodTargetLabels: []string{"a", "b", "c/d/e"},
				},
			}, v1.Endpoint{}, 0)

			assertRLCWith(t, cfg.ScrapeConfigs[0], rlcMatchSingle("__meta_kubernetes_pod_label_a"), targetLabelTest("a"))
			assertRLCWith(t, cfg.ScrapeConfigs[0], rlcMatchSingle("__meta_kubernetes_pod_label_b"), targetLabelTest("b"))
			assertRLCWith(t, cfg.ScrapeConfigs[0], rlcMatchSingle("__meta_kubernetes_pod_label_c_d_e"), targetLabelTest("c_d_e"))
		})

		t.Run("Default Job RLC", func(t *testing.T) {
			cfg := genConfig(sut, v1.Endpoint{})

			assertRLCWith(t, cfg.ScrapeConfigs[0], func(rlc *relabel.Config) bool {
				return rlcMatchSingle("__meta_kubernetes_service_name")(rlc) && rlc.TargetLabel == "job"
			}, func(t *testing.T, rlc *relabel.Config) {
				assert.Equal(t, "${1}", rlc.Replacement)
			})
		})

		t.Run("Job Label", func(t *testing.T) {
			cfg := sut.makeInstanceForServiceMonitorEndpoint(&v1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy",
					Namespace: "myapp",
				},
				Spec: v1.ServiceMonitorSpec{
					Endpoints: []v1.Endpoint{},
					JobLabel:  "foo.bar/app",
				},
			}, v1.Endpoint{}, 0)

			assertRLCWith(t, cfg.ScrapeConfigs[0], rlcMatchSingle("__meta_kubernetes_service_label_foo_bar_app"), func(t *testing.T, rlc *relabel.Config) {
				assert.Equal(t, "job", rlc.TargetLabel)
				assert.Equal(t, "^(?:(.+))$", rlc.Regex.String())
				assert.Equal(t, "${1}", rlc.Replacement)
			})
		})

		t.Run("Endpoint Port RLC", func(t *testing.T) {
			match := func(replacement string) func(rlc *relabel.Config) bool {
				return func(rlc *relabel.Config) bool {
					return rlc.TargetLabel == "endpoint" && rlc.Replacement == replacement
				}
			}

			t.Run("Port", func(t *testing.T) {
				cfg := genConfig(sut, v1.Endpoint{Port: "metrics"})

				assertRLCWith(t, cfg.ScrapeConfigs[0], match("metrics"), func(_ *testing.T, _ *relabel.Config) {})
			})

			t.Run("Target Port", func(t *testing.T) {
				t.Run("String", func(t *testing.T) {
					v := intstr.FromString("metrics")
					cfg := genConfig(sut, v1.Endpoint{TargetPort: &v})

					assertRLCWith(t, cfg.ScrapeConfigs[0], match("metrics"), func(_ *testing.T, _ *relabel.Config) {})
				})

				t.Run("Int", func(t *testing.T) {
					v := intstr.FromInt(9000)
					cfg := genConfig(sut, v1.Endpoint{TargetPort: &v})

					assertRLCWith(t, cfg.ScrapeConfigs[0], match("9000"), func(_ *testing.T, _ *relabel.Config) {})
				})
			})
		})

		testRLCs := []*v1.RelabelConfig{
			{SourceLabels: []string{"s1"}, Replacement: "r1", TargetLabel: "t1", Separator: "sep1", Action: "keep", Modulus: 123},
			{SourceLabels: []string{"s2"}, Replacement: "r2", TargetLabel: "t2", Separator: "sep2", Action: "drop", Modulus: 456},
			{SourceLabels: []string{"s3"}, Replacement: "r3", TargetLabel: "t3", Separator: "sep3", Action: "keep", Modulus: 789, Regex: "regex"},
			{SourceLabels: []string{"s4", "s5"}, Replacement: "r45", TargetLabel: "t45", Separator: "sep45", Action: "keep", Modulus: 112233},
		}

		rlcCheck := func(t *testing.T, cfg *instance.Config) {
			assertRLCWith(t, cfg.ScrapeConfigs[0], rlcMatchSingle("s1"), func(t *testing.T, rlc *relabel.Config) {
				assert.Equal(t, "r1", rlc.Replacement)
				assert.Equal(t, "t1", rlc.TargetLabel)
				assert.Equal(t, "sep1", rlc.Separator)
				assert.Equal(t, relabel.Keep, rlc.Action)
				assert.Equal(t, uint64(123), rlc.Modulus)
			})

			assertRLCWith(t, cfg.ScrapeConfigs[0], rlcMatchSingle("s2"), func(t *testing.T, rlc *relabel.Config) {
				assert.Equal(t, "r2", rlc.Replacement)
				assert.Equal(t, "t2", rlc.TargetLabel)
				assert.Equal(t, "sep2", rlc.Separator)
				assert.Equal(t, relabel.Drop, rlc.Action)
				assert.Equal(t, uint64(456), rlc.Modulus)
			})

			assertRLCWith(t, cfg.ScrapeConfigs[0], rlcMatchSingle("s3"), func(t *testing.T, rlc *relabel.Config) {
				assert.Equal(t, "r3", rlc.Replacement)
				assert.Equal(t, "t3", rlc.TargetLabel)
				assert.Equal(t, "sep3", rlc.Separator)
				assert.Equal(t, relabel.Keep, rlc.Action)
				assert.Equal(t, uint64(789), rlc.Modulus)
				assert.Equal(t, "^(?:regex)$", rlc.Regex.String())
			})

			assertRLCWith(t, cfg.ScrapeConfigs[0], func(rlc *relabel.Config) bool {
				return len(rlc.SourceLabels) == 2 && rlc.SourceLabels[0] == "s4" && rlc.SourceLabels[1] == "s5"
			}, func(t *testing.T, rlc *relabel.Config) {
				assert.Equal(t, "r45", rlc.Replacement)
				assert.Equal(t, "t45", rlc.TargetLabel)
				assert.Equal(t, "sep45", rlc.Separator)
				assert.Equal(t, relabel.Keep, rlc.Action)
				assert.Equal(t, uint64(112233), rlc.Modulus)
			})
		}

		t.Run("Endpoint RLC", func(t *testing.T) {
			rlcCheck(t, genConfig(sut, v1.Endpoint{RelabelConfigs: testRLCs}))
		})

		t.Run("Endpoint Metric RLC", func(t *testing.T) {
			rlcCheck(t, genConfig(sut, v1.Endpoint{MetricRelabelConfigs: testRLCs}))
		})
	})
}

func rlcMatchSingle(source string) func(rlc *relabel.Config) bool {
	return func(rlc *relabel.Config) bool {
		return len(rlc.SourceLabels) == 1 && string(rlc.SourceLabels[0]) == source
	}
}

func assertRLC(t *testing.T, sc *config.ScrapeConfig, action relabel.Action, source, regex string) {
	assertRLCWith(t, sc, rlcMatchSingle(source), func(t *testing.T, rlc *relabel.Config) {
		assert.Equal(t, action, rlc.Action)
		assert.Equal(t, regex, rlc.Regex.String())
	})
}

func assertRLCTarget(t *testing.T, sc *config.ScrapeConfig, source, target string) {
	assertRLCWith(t, sc, func(rlc *relabel.Config) bool {
		return rlcMatchSingle(source)(rlc) && rlc.TargetLabel == target
	}, func(t *testing.T, rlc *relabel.Config) {})
}

func assertRLCWith(t *testing.T, sc *config.ScrapeConfig, match func(rlc *relabel.Config) bool, test func(t *testing.T, rlc *relabel.Config)) {
	for _, rlc := range sc.RelabelConfigs {
		if match(rlc) {
			test(t, rlc)
			return
		}
	}

	t.Errorf("an expected relabel config was not found")
}
