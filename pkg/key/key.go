package key

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/microerror"

	"github.com/giantswarm/capa-iam-operator/pkg/iam"
)

const (
	ClusterNameLabel        = "cluster.x-k8s.io/cluster-name"
	ClusterWatchFilterLabel = "cluster.x-k8s.io/watch-filter"
	ClusterRole             = "cluster.x-k8s.io/role"
)

func FinalizerName(roleName string) string {
	return fmt.Sprintf("capa-iam-operator.finalizers.giantswarm.io/%s", roleName)
}

func GetClusterIDFromLabels(t v1.ObjectMeta) (string, error) {
	value := t.GetLabels()[ClusterNameLabel]
	if value == "" {
		return "", fmt.Errorf("missing label %q", ClusterNameLabel)
	}
	return value, nil
}

func GetClusterByName(ctx context.Context, ctrlClient client.Client, clusterName string, namespace string) (*capi.Cluster, error) {
	cluster := &capi.Cluster{}

	if err := ctrlClient.Get(ctx, types.NamespacedName{
		Name:      clusterName,
		Namespace: namespace,
	}, cluster); err != nil {
		return nil, err
	}

	return cluster, nil
}

func GetAWSClusterByName(ctx context.Context, ctrlClient client.Client, clusterName string, namespace string) (*capa.AWSCluster, error) {
	awsClusterList := &capa.AWSClusterList{}

	if err := ctrlClient.List(ctx,
		awsClusterList,
		client.InNamespace(namespace),
		client.MatchingLabels{ClusterNameLabel: clusterName},
	); err != nil {
		return nil, err
	}

	if len(awsClusterList.Items) != 1 {
		return nil, fmt.Errorf("expected 1 AWSCluster but found %d", len(awsClusterList.Items))
	}

	return &awsClusterList.Items[0], nil
}

func GetAWSClusterRoleIdentity(ctx context.Context, ctrlClient client.Client, awsClusterRoleIdentityName string) (*capa.AWSClusterRoleIdentity, error) {
	awsClusterRoleIdentity := &capa.AWSClusterRoleIdentity{}

	if err := ctrlClient.Get(ctx, types.NamespacedName{
		Name:      awsClusterRoleIdentityName,
		Namespace: "",
	}, awsClusterRoleIdentity); err != nil {
		return nil, err
	}

	return awsClusterRoleIdentity, nil
}

func HasCapiWatchLabel(labels map[string]string) bool {
	value, ok := labels[ClusterWatchFilterLabel]
	if ok {
		if value == "capi" {
			return true
		}
	}
	return false
}

func IsControlPlaneAWSMachineTemplate(labels map[string]string) bool {
	value, ok := labels[ClusterRole]
	if ok {
		if value == iam.ControlPlaneRole {
			return true
		}
	}
	return false
}

func IsBastionAWSMachineTemplate(labels map[string]string) bool {
	value, ok := labels[ClusterRole]
	if ok {
		if value == iam.BastionRole {
			return true
		}
	}
	return false
}

func CloudFrontAlias(baseDomain string) string {
	return fmt.Sprintf("irsa.%s", baseDomain)
}

func BaseDomain(cluster capi.Cluster) (string, error) {
	apiEndpoint := cluster.Spec.ControlPlaneEndpoint.Host
	if apiEndpoint == "" {
		return "", microerror.Mask(missingApiEndpointError)
	}
	if !strings.HasPrefix(apiEndpoint, "api.") {
		return "", microerror.Mask(unexpectedApiEndpointError)
	}
	return strings.TrimPrefix(apiEndpoint, "api."), nil
}
