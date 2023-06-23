package awsclient

import (
	"context"
	"errors"

	clientaws "github.com/aws/aws-sdk-go/aws/client"
	"github.com/go-logr/logr"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/capa-iam-operator/pkg/key"
)

type AwsClientInterface interface {
	GetAWSClientSession(ctx context.Context, clusterName, namespace string) (clientaws.ConfigProvider, error)
}

type AWSClientConfig struct {
	CtrlClient client.Client
	Log        logr.Logger
}

type AwsClient struct {
	ctrlClient client.Client
	log        logr.Logger
}

func New(config AWSClientConfig) (*AwsClient, error) {
	if config.CtrlClient == nil {
		return nil, errors.New("failed to generate new awsClient from nil CtrlClient")
	}

	a := &AwsClient{
		ctrlClient: config.CtrlClient,
		log:        config.Log,
	}

	return a, nil
}

func (a *AwsClient) GetAWSClientSession(ctx context.Context, clusterName, namespace string) (clientaws.ConfigProvider, error) {
	awsCluster, err := key.GetAWSClusterByName(ctx, a.ctrlClient, clusterName, namespace)
	if err != nil {
		a.log.Error(err, "failed to get AWSCluster")
		return nil, err
	}

	cluster, err := capiutil.GetClusterFromMetadata(ctx, a.ctrlClient, awsCluster.ObjectMeta)
	if err != nil {
		return nil, err
	}

	// Create the cluster scope just to reuse logic of getting proper AWS session from cluster-api-provider-aws controller code
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:         a.ctrlClient,
		Logger:         &a.log,
		Cluster:        cluster,
		AWSCluster:     awsCluster,
		ControllerName: "capa-iam",
	})
	if err != nil {
		return nil, err
	}

	return clusterScope.Session(), nil
}
