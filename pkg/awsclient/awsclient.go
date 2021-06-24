package awsclient

import (
	"context"
	"errors"

	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/go-logr/logr"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
	corev1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AWSClientConfig struct {
	Cluster    *corev1.Cluster
	CtrlClient client.Client
	Ctx        context.Context
	Log        logr.Logger
}

type AwsClient struct {
	cluster    *corev1.Cluster
	ctrlClient client.Client
	ctx        context.Context
	log        logr.Logger
}

func New(config AWSClientConfig) (*AwsClient, error) {
	if config.Cluster == nil {
		return nil, errors.New("failed to generate new awsClient from nil Cluster")
	}
	if config.CtrlClient == nil {
		return nil, errors.New("failed to generate new awsClient from nil CtrlClient")
	}
	if config.Log == nil {
		return nil, errors.New("failed to generate new awsClient from nil Log")
	}

	a := &AwsClient{
		cluster:    config.Cluster,
		ctrlClient: config.CtrlClient,
		ctx:        config.Ctx,
		log:        config.Log,
	}

	return a, nil
}

func (a *AwsClient) GetAWSClientSession() (awsclient.ConfigProvider, error) {
	awsCluster := &infrav1.AWSCluster{}

	awsClusterName := client.ObjectKey{
		Namespace: a.cluster.Namespace,
		Name:      a.cluster.Spec.InfrastructureRef.Name,
	}

	if err := a.ctrlClient.Get(a.ctx, awsClusterName, awsCluster); err != nil {
		// AWSCluster is not ready
		a.log.V(5).Info("AWSCluster not found yet")
		return nil, err // nolint:nilerr
	}

	// Create the cluster scope just to reuse logic of getting proper AWS session from cluster-api-provider-aws controller code
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:         a.ctrlClient,
		Logger:         a.log,
		Cluster:        a.cluster,
		AWSCluster:     awsCluster,
		ControllerName: "iam",
	})
	if err != nil {
		return nil, err
	}

	return clusterScope.Session(), nil
}
