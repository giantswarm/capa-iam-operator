package awsclient

import (
	"context"
	"errors"
	"fmt"

	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/go-logr/logr"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/scope"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/capa-iam-controller/pkg/key"
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
	if config.ClusterName == "nil" {
		return nil, errors.New("failed to generate new awsClient from empty ClusterName")
	}
	if config.CtrlClient == nil {
		return nil, errors.New("failed to generate new awsClient from nil CtrlClient")
	}
	if config.Log == nil {
		return nil, errors.New("failed to generate new awsClient from nil Log")
	}

	a := &AwsClient{
		clusterName: config.ClusterName,
		ctrlClient:  config.CtrlClient,
		log:         config.Log,
	}

	return a, nil
}

func (a *AwsClient) GetAWSClientSession(ctx context.Context) (awsclient.ConfigProvider, error) {
	awsClusterList := &capa.AWSClusterList{}

	if err := a.ctrlClient.List(ctx,
		awsClusterList,
		client.MatchingLabels{key.ClusterNameLabel: a.clusterName},
	); err != nil {
		a.log.Error(err, "cannot fetch AWSClusters")
		return nil, err // nolint:nilerr
	}

	if len(awsClusterList.Items) != 1 {
		// AWSCluster is not ready
		a.log.Info(fmt.Sprintf("expected 1 AWSCluster but found '%d'", len(awsClusterList.Items)))
		return nil, nil // nolint:nilerr
	}

	cluster, err := capiutil.GetClusterFromMetadata(ctx, a.ctrlClient, awsClusterList.Items[0].ObjectMeta)
	if err != nil {
		return nil, err
	}

	// Create the cluster scope just to reuse logic of getting proper AWS session from cluster-api-provider-aws controller code
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:         a.ctrlClient,
		Logger:         a.log,
		Cluster:        cluster,
		AWSCluster:     &awsClusterList.Items[0],
		ControllerName: "capa-iam",
	})
	if err != nil {
		return nil, err
	}

	return clusterScope.Session(), nil
}
