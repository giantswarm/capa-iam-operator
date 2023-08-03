package awsclient

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	clientaws "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AwsClientInterface interface {
	GetAWSClientSession(awsRoleARN string, region string) (clientaws.ConfigProvider, error)
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

func (a *AwsClient) GetAWSClientSession(awsRoleARN string, region string) (clientaws.ConfigProvider, error) {
	s, err := sessionForRegion(region)
	if err != nil {
		return nil, microerror.Mask(err)
	}
	awsClientConfig := &aws.Config{Credentials: stscreds.NewCredentials(s, awsRoleARN)}

	o, err := session.NewSession(awsClientConfig)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return o, nil
}

func sessionForRegion(region string) (*session.Session, error) {
	ns, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, err
	}

	return ns, nil
}
