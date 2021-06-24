module github.com/giantswarm/capa-iam-controller

go 1.16

require (
<<<<<<< HEAD
<<<<<<< HEAD
	github.com/aws/aws-sdk-go v1.36.26
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
=======
	github.com/aws/aws-sdk-go v1.36.26
	github.com/go-logr/logr v0.1.0
>>>>>>> 4af228ac55359932b8062d0980919829f37f9932
	k8s.io/api v0.17.9
	k8s.io/apimachinery v0.17.9
	k8s.io/client-go v0.17.9
	k8s.io/klog v1.0.0
	sigs.k8s.io/cluster-api v0.3.19
	sigs.k8s.io/cluster-api-provider-aws v0.6.6
	sigs.k8s.io/controller-runtime v0.5.14
)

replace (
	github.com/coreos/etcd v3.3.10+incompatible => github.com/coreos/etcd v3.3.25+incompatible
	github.com/gogo/protobuf v1.3.1 => github.com/gogo/protobuf v1.3.2
	github.com/gorilla/websocket v1.4.0 => github.com/gorilla/websocket v1.4.2
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)
