local k = import 'ksonnet-util/kausal.libsonnet';

local container = k.core.v1.container;
local containerPort = k.core.v1.containerPort;
local deployment = k.apps.v1.deployment;
local service = k.core.v1.service;

local service_labels = {
   app: 'example',
};

{
  app(name, namespace):: {
    container::
      container.new('app', 'quay.io/brancz/prometheus-example-app:v0.3.0') +
      container.withPorts([
        containerPort.newNamed(name='http', containerPort=8080),
      ]),

    deployment:
      deployment.new(name, 1, [self.container]) +
      deployment.mixin.metadata.withNamespace(namespace),

    service:
      k.util.serviceFor(self.deployment) +
      service.mixin.metadata.withNamespace(namespace) +
      service.mixin.metadata.withLabelsMixin(service_labels),

    service_monitor: {
        apiVersion: 'monitoring.coreos.com/v1',
        kind: 'ServiceMonitor',
        metadata: {
            name: name,
            namespace: namespace,
        },
        spec: {
            jobLabel: 'app',
            selector: {
                matchLabels: service_labels,
            },
            endpoints: [
                {
                    port: 'app-http',
                }
            ]
        },
    }
  },
}
