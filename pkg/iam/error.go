package iam

import (
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
	"github.com/giantswarm/microerror"
)

var invalidClusterError = &microerror.Error{
	Kind: "invalidClusterError",
}

func IsNotFound(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == awsiam.ErrCodeNoSuchEntityException {
			return true
		}
	}
	return false
}

func IsAlreadyExists(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == awsiam.ErrCodeEntityAlreadyExistsException {
			return true
		}
	}
	return false
}
