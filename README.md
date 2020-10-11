# grafana-agent-operator

An experimental operator to watch for [`ServiceMonitor`](https://github.com/prometheus-operator/prometheus-operator/blob/master/Documentation/api.md#servicemonitor)s.

Highly experimental and WIP

## Building

You need a recent version of Go for Go Modules support.

## Usage

The operator should be deployed in each cluster you wish to monitor, alongside a clustered
deployment of the grafana agent running in [`Scraping Service` mode](https://github.com/grafana/agent/blob/master/docs/scraping-service.md).

Each [`Endpoint`]() in each discovered [`ServiceMonitor`](https://github.com/prometheus-operator/prometheus-operator/blob/master/Documentation/api.md#servicemonitor)
will render a single [`Instance`](https://github.com/grafana/agent/blob/master/docs/configuration-reference.md#prometheus_instance_config)
for the agent to monitor to maximize sharding.

