module github.com/giantswarm/capa-iam-operator

go 1.16

require (
	github.com/Azure/go-autorest/autorest v0.11.18 // indirect
	github.com/aws/aws-sdk-go v1.40.53
	github.com/go-logr/logr v0.1.0
	github.com/google/go-cmp v0.5.6
	k8s.io/api v0.17.17
	k8s.io/apimachinery v0.17.17
	k8s.io/client-go v0.17.17
	k8s.io/klog v1.0.0
	sigs.k8s.io/cluster-api v0.3.22
	sigs.k8s.io/cluster-api-provider-aws v0.6.8
	sigs.k8s.io/controller-runtime v0.5.14
)

replace (
	github.com/containernetworking/cni v0.8.0 => github.com/containernetworking/cni v1.1.1
	github.com/coreos/etcd v3.3.10+incompatible => github.com/coreos/etcd v3.3.25+incompatible
	github.com/dgrijalva/jwt-go => github.com/dgrijalva/jwt-go/v4 v4.0.0-preview1
	github.com/gogo/protobuf v1.3.1 => github.com/gogo/protobuf v1.3.2
	github.com/gorilla/websocket v1.4.0 => github.com/gorilla/websocket v1.4.2
	github.com/miekg/dns v1.0.14 => github.com/miekg/dns v1.1.50

	github.com/pkg/sftp v1.10.1 => github.com/pkg/sftp v1.13.5
	github.com/prometheus/client_golang v1.11.0 => github.com/prometheus/client_golang v1.12.2
	go.mongodb.org/mongo-driver v1.1.2 => go.mongodb.org/mongo-driver v1.5.1
	golang.org/x/text v0.3.6 => golang.org/x/text v0.3.7
)
