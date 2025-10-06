package awsclient

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AwsClientInterface interface {
	GetAWSClientConfig(awsRoleARN string, region string) (aws.Config, error)
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

// GetAWSClientConfig doesn't use the receiver argument, so I don't know why this is a method.
func (a *AwsClient) GetAWSClientConfig(awsRoleARN string, region string) (aws.Config, error) {
	// Initial credentials loaded from SDK's default credential chain. Such as
	// the environment, shared credentials (~/.aws/credentials), or EC2 Instance
	// Role. These credentials will be used to to make the STS Assume Role API.
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return aws.Config{}, microerror.Mask(err)
	}

	// Create the credentials from AssumeRoleProvider to assume the role
	// referenced by the "awsRoleARN" ARN.
	stsSvc := sts.NewFromConfig(cfg)
	creds := stscreds.NewAssumeRoleProvider(stsSvc, awsRoleARN)

	cfg.Credentials = aws.NewCredentialsCache(creds)

	return cfg, nil
}
