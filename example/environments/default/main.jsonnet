local k = import 'ksonnet-util/kausal.libsonnet';

local agent   = import 'grafana-agent/scraping-svc/main.libsonnet';
local etcd    = import 'etcd.libsonnet';
local cortex  = import 'cortex/main.libsonnet';
local grafana = import 'grafana/main.libsonnet';
local example = import 'example.libsonnet';

local datasource = import 'grafana/datasource.libsonnet';

local namespace = k.core.v1.namespace;

local namespaced(obj, namespace) = {
    [k]: obj[k] + {
      [if std.objectHas(obj[k], 'kind') && !std.startsWith(obj[k].kind, 'Cluster') then 'metadata']+: {
          namespace: 'monitoring'
      }
  }
    for k in std.objectFields(obj)
};

local agent_objects = agent.new(namespace='monitoring') +
  agent.withConfigMixin({
      local kvstore = {
          store: 'etcd',
          etcd: {
              endpoints: ['etcd.etcd.svc.cluster.local:2379']
          }
      },

      agent_ring_kvstore: kvstore { prefix: 'agent/ring/' },
      agent_config_kvstore: kvstore { prefix: 'agent/configs/' },

      agent_remote_write: [
          {
              url: 'http://cortex.monitoring.svc.cluster.local/api/prom/push'
          }
      ],

      agent_config+: {
          prometheus+: {
              global+: {
                  external_labels+: {
                      cluster: 'dev'
                  }
              }
          }
      }
  }) + {
    // Disable the syncer since our operator will be syncing configs
    syncer: {},
  };

// The agent manifests seem to ignore _config.namespace. Force them to `monitoring`.
local namespaced_agent_objects = namespaced(agent_objects, 'monitoring') + {
    rbac: namespaced(agent_objects.rbac, 'monitoring')
};

local grafana_objects = grafana.new(namespace='monitoring') +
    grafana.withDashboards((import 'cortex-mixin/dashboards.jsonnet')) +
    grafana.withDataSources([
        datasource.new('Cortex', 'http://cortex.monitoring.svc.cluster.local/api/prom')
    ]);

local namespaced_grafana_objects = namespaced(grafana_objects, 'monitoring') + {
    grafana_dashboard_cms: namespaced(grafana_objects.grafana_dashboard_cms, 'monitoring')
};

{
    namespaces: {
        etcd: namespace.new('etcd'),
        monitoring: namespace.new('monitoring'),
        example_apps: {
            a: namespace.new('foo-a'),
            b: namespace.new('foo-b'),
            c: namespace.new('foo-c'),
        }
    },
    crds: {
        ServiceMonitor: (import 'prometheus-operator/servicemonitor-crd.libsonnet'),
    },
    etcd: etcd.new(namespace='etcd'),
    agent: namespaced_agent_objects,
    grafana: namespaced_grafana_objects,
    cortex: cortex.new(namespace='monitoring') + {
        service_monitor: {
           apiVersion: 'monitoring.coreos.com/v1',
           kind: 'ServiceMonitor',
           metadata: {
               name: 'cortex',
               namespace: 'monitoring',
           },
           spec: {
               jobLabel: 'name',
               selector: {
                   matchLabels: {name: 'cortex'},
               },
               endpoints: [
                   {
                       port: 'cortex-http-metrics',
                   }
               ]
           },
        }
    },
    example_aps: {
        a: example.app('example', 'foo-a'),
        b: example.app('example', 'foo-b'),
        c: example.app('example', 'foo-c'),
    },
}
