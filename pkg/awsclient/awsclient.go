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

type AWSClientConfig struct {
	ClusterName string
	CtrlClient  client.Client
	Log         logr.Logger
}

type AwsClient struct {
	clusterName string
	ctrlClient  client.Client
	log         logr.Logger
}

func New(config AWSClientConfig) (*AwsClient, error) {
	if config.ClusterName == "" {
		return nil, errors.New("failed to generate new awsClient from empty ClusterName")
	}
	if config.CtrlClient == nil {
		return nil, errors.New("failed to generate new awsClient from nil CtrlClient")
	}

	a := &AwsClient{
		clusterName: config.ClusterName,
		ctrlClient:  config.CtrlClient,
		log:         config.Log,
	}

	return a, nil
}

func (a *AwsClient) GetAWSClientSession(ctx context.Context) (clientaws.ConfigProvider, error) {
	awsCluster, err := key.GetAWSClusterByName(ctx, a.ctrlClient, a.clusterName)
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
