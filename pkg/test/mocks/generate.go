//go:generate ../../../tools/mockgen -destination aws_iam_mock.go -package mocks github.com/aws/aws-sdk-go/service/iam/iamiface IAMAPI

package mocks
