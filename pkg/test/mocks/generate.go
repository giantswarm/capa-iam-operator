//go:generate ../../../tools/mockgen -destination aws_iam_mock.go -package mocks github.com/aws/aws-sdk-go/service/iam/iamiface IAMAPI
//go:generate ../../../tools/mockgen -destination awsclient_mock.go -package mocks -source ../../awsclient/awsclient.go AWSClient
package mocks
