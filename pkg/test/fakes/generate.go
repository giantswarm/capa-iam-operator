package fakes

import (
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate -o awssdkfakes . IAMAPI
type IAMAPI interface {
	iamiface.IAMAPI
}
