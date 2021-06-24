package iam

import (
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
)

func IsNotFound(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == awsiam.ErrCodeNoSuchEntityException {
			return true
		}
	}
	return false
}
