package key

import (
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
)

const (
	CAPAIAMControllerFinalizer = "capa-iam-controller.finalizers.giantswarm.io"

	ClusterNameLabel = "cluster.x-k8s.io/cluster-name"
)

func GetClusterIDFromLabels(t *capa.AWSMachineTemplate) string {
	return t.GetLabels()[ClusterNameLabel]
}
