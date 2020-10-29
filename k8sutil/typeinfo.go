package k8sutil

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

func init() {
	utilruntime.Must(monitoringv1.AddToScheme(scheme.Scheme))
}

// https://github.com/kubernetes/client-go/issues/308#issuecomment-700099260
func AddTypeMetaToObject(obj runtime.Object) error {
	groups, _, err := scheme.Scheme.ObjectKinds(obj)
	if err != nil {
		return fmt.Errorf("missing apiVersion or kind: %w", err)
	}

	for _, group := range groups {
		if len(group.Kind) == 0 {
			continue
		}

		if len(group.Version) == 0 || group.Version == runtime.APIVersionInternal {
			continue
		}

		obj.GetObjectKind().SetGroupVersionKind(group)
	}

	return nil
}
