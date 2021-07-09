package key

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CAPAIAMControllerFinalizer = "capa-iam-controller.finalizers.giantswarm.io"

	ClusterNameLabel = "cluster.x-k8s.io/cluster-name"
)

func GetClusterIDFromLabels(t v1.ObjectMeta) string {
	return t.GetLabels()[ClusterNameLabel]
}
