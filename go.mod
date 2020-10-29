module github.com/nlowe/grafana-agent-operator

go 1.15

replace (
	// Required to fix a restore error with the grafana agent
	// https://github.com/grafana/agent/blob/60e9cbb63c6bb5ca8fa70b9a774ee969f2c65f85/go.mod#L62-L63
	github.com/ncabatoff/process-exporter => github.com/grafana/process-exporter v0.7.3-0.20200902205007-6343dc1182cf
	// https://github.com/grafana/agent/blob/60e9cbb63c6bb5ca8fa70b9a774ee969f2c65f85/go.mod#L58
	github.com/prometheus/prometheus => github.com/grafana/prometheus v1.8.2-0.20200821135656-2efe42db3b77
	// https://github.com/grafana/agent/blob/60e9cbb63c6bb5ca8fa70b9a774ee969f2c65f85/go.mod#L60
	gopkg.in/yaml.v2 => github.com/rfratto/go-yaml v0.0.0-20200521142311-984fc90c8a04

	// Required to fix a restore error with the prometheus-operator
	k8s.io/client-go => k8s.io/client-go v0.18.8
)

require (
	github.com/grafana/agent v0.6.1
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/magiconair/properties v1.8.4 // indirect
	github.com/mattn/go-colorable v0.1.8
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/prometheus-operator/prometheus-operator v0.42.2-0.20200928114327-fbd01683839a
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.42.1
	github.com/prometheus/common v0.11.1
	github.com/prometheus/prometheus v2.5.0+incompatible
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/afero v1.4.1 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.5.1
	github.com/x-cray/logrus-prefixed-formatter v0.5.2
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0 // indirect
	golang.org/x/sys v0.0.0-20201009025420-dfb3f7c4e634 // indirect
	gopkg.in/ini.v1 v1.62.0 // indirect
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v12.0.0+incompatible
)
