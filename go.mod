module github.com/giantswarm/capa-iam-controller

go 1.16

require (
	github.com/aws/aws-sdk-go v1.36.26
	github.com/go-logr/logr v0.2.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/cluster-api v0.3.19
	sigs.k8s.io/cluster-api-provider-aws v0.6.6
	sigs.k8s.io/controller-runtime v0.6.4
)

replace (
	github.com/coreos/etcd v3.3.10+incompatible => github.com/coreos/etcd v3.3.25+incompatible
	github.com/gogo/protobuf v1.3.1 => github.com/gogo/protobuf v1.3.2
	github.com/gorilla/websocket v1.4.0 => github.com/gorilla/websocket v1.4.2
)
