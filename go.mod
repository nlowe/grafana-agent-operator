module github.com/nlowe/grafana-agent-operator

go 1.15

replace (
	// https://github.com/grafana/agent/blob/e9bd098ffd09930015173008ffb254018e7333c4/go.mod#L67-L74
	github.com/google/dnsmasq_exporter => github.com/grafana/dnsmasq_exporter v0.2.1-0.20201029182940-e5169b835a23
	github.com/ncabatoff/process-exporter => github.com/grafana/process-exporter v0.7.3-0.20210106202358-831154072e2a
	github.com/prometheus/mysqld_exporter => github.com/grafana/mysqld_exporter v0.12.2-0.20201015182516-5ac885b2d38a
	github.com/wrouesnel/postgres_exporter => github.com/grafana/postgres_exporter v0.8.1-0.20201106170118-5eedee00c1db
)

replace (
	// Required to fix a restore error with the grafana agent
	// https://github.com/grafana/agent/blob/e9bd098ffd09930015173008ffb254018e7333c4/go.mod#L80-L86
	github.com/Azure/azure-sdk-for-go => github.com/Azure/azure-sdk-for-go v36.2.0+incompatible
	github.com/hashicorp/consul => github.com/hashicorp/consul v1.5.1
	github.com/hpcloud/tail => github.com/grafana/tail v0.0.0-20201004203643-7aa4e4a91f03
	k8s.io/api => k8s.io/api v0.19.4
	k8s.io/client-go => k8s.io/client-go v0.19.4
)

replace (
	// https://github.com/grafana/agent/blob/e9bd098ffd09930015173008ffb254018e7333c4/go.mod#L76-L78
	github.com/prometheus/prometheus => github.com/grafana/prometheus v1.8.2-0.20210218144103-50bc1c15f0c7
	gopkg.in/yaml.v2 => github.com/rfratto/go-yaml v0.0.0-20200521142311-984fc90c8a04
)

require (
	github.com/grafana/agent v0.13.0
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/magiconair/properties v1.8.4 // indirect
	github.com/mattn/go-colorable v0.1.8
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.46.0
	github.com/prometheus-operator/prometheus-operator/pkg/client v0.46.0
	github.com/prometheus/common v0.15.0
	github.com/prometheus/prometheus v2.5.0+incompatible
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/afero v1.4.1 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/x-cray/logrus-prefixed-formatter v0.5.2
	gopkg.in/ini.v1 v1.62.0 // indirect
	k8s.io/api v0.20.2
	k8s.io/apiextensions-apiserver v0.20.1 // indirect
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v12.0.0+incompatible
)
