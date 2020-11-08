module github.com/nlowe/grafana-agent-operator

go 1.15

replace (
	// Required to fix a restore error with the grafana agent
	// https://github.com/grafana/agent/blob/5c59f84342a99761b6108b4f68d4f596158d7bc3/go.mod#L68-L74
	github.com/google/dnsmasq_exporter => github.com/grafana/dnsmasq_exporter v0.2.1-0.20201029182940-e5169b835a23
	github.com/ncabatoff/process-exporter => github.com/grafana/process-exporter v0.7.3-0.20200902205007-6343dc1182cf
	github.com/prometheus/memcached_exporter => github.com/grafana/memcached_exporter v0.7.1-0.20201030142623-8e1997d4fbb7
	github.com/prometheus/mysqld_exporter => github.com/grafana/mysqld_exporter v0.12.2-0.20201015182516-5ac885b2d38a

	// https://github.com/grafana/agent/blob/5c59f84342a99761b6108b4f68d4f596158d7bc3/go.mod#L64
	github.com/prometheus/prometheus => github.com/grafana/prometheus v1.8.2-0.20201021200247-cf00050ed1e9
	// https://github.com/grafana/agent/blob/5c59f84342a99761b6108b4f68d4f596158d7bc3/go.mod#L66
	gopkg.in/yaml.v2 => github.com/rfratto/go-yaml v0.0.0-20200521142311-984fc90c8a04

	// Required to fix a restore error with the prometheus-operator
	k8s.io/client-go => k8s.io/client-go v0.19.2
)

require (
	github.com/grafana/agent v0.8.0
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/magiconair/properties v1.8.4 // indirect
	github.com/mattn/go-colorable v0.1.8
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/prometheus-operator/prometheus-operator v0.43.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.43.0
	github.com/prometheus/common v0.14.0
	github.com/prometheus/prometheus v2.5.0+incompatible
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/afero v1.4.1 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	github.com/x-cray/logrus-prefixed-formatter v0.5.2
	gopkg.in/ini.v1 v1.62.0 // indirect
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v12.0.0+incompatible
)
