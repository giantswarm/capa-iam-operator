//go:generate ../../../tools/mockgen -destination aws_iam_eks_mock.go -package mocks -source ../../iam/iam.go IAMClient
//go:generate ../../../tools/mockgen -destination awsclient_mock.go -package mocks -source ../../awsclient/awsclient.go AWSClient

package mocks
