package releaseinspector

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReleaseInspector struct {
	kubeClient client.Client
}