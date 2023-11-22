module github.com/giantswarm/capa-iam-operator

go 1.16

require (
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d // indirect
	github.com/aws/aws-sdk-go v1.48.3
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/giantswarm/microerror v0.4.1
	github.com/go-logr/logr v1.3.0
	github.com/golang/mock v1.6.0
	github.com/google/uuid v1.4.0
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/onsi/ginkgo/v2 v2.13.1
	github.com/onsi/gomega v1.30.0
	github.com/pkg/errors v0.9.1
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/oauth2 v0.0.0-20221014153046-6fdb5e3db783 // indirect
	google.golang.org/genproto v0.0.0-20221014173430-6e2ab493f96b // indirect
	k8s.io/api v0.25.0
	k8s.io/apimachinery v0.25.0
	k8s.io/client-go v0.25.0
	k8s.io/klog/v2 v2.80.1
	k8s.io/kubectl v0.26.2
	sigs.k8s.io/cluster-api v1.3.1
	sigs.k8s.io/cluster-api-provider-aws v1.5.2
	sigs.k8s.io/controller-runtime v0.13.1
)

replace (
	github.com/containernetworking/cni v0.8.0 => github.com/containernetworking/cni v1.1.1
	github.com/coreos/etcd v3.3.10+incompatible => github.com/coreos/etcd v3.3.25+incompatible
	github.com/dgrijalva/jwt-go => github.com/dgrijalva/jwt-go/v4 v4.0.0-preview1
	github.com/gogo/protobuf v1.3.1 => github.com/gogo/protobuf v1.3.2
	github.com/gorilla/websocket v1.4.0 => github.com/gorilla/websocket v1.4.2
	github.com/hashicorp/consul => github.com/hashicorp/consul v1.17.0
	github.com/miekg/dns v1.0.14 => github.com/miekg/dns v1.1.50
	github.com/pkg/sftp v1.10.1 => github.com/pkg/sftp v1.13.5
	github.com/prometheus/client_golang v1.11.0 => github.com/prometheus/client_golang v1.12.2
	go.mongodb.org/mongo-driver v1.1.2 => go.mongodb.org/mongo-driver v1.5.1
	golang.org/x/text v0.3.6 => golang.org/x/text v0.3.8
	golang.org/x/text v0.3.7 => golang.org/x/text v0.3.8

	k8s.io/api => k8s.io/api v0.25.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.25.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.25.0
	k8s.io/apiserver => k8s.io/apiserver v0.25.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.25.0
	k8s.io/client-go => k8s.io/client-go v0.25.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.25.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.25.0
	k8s.io/code-generator => k8s.io/code-generator v0.22.2
	k8s.io/component-base => k8s.io/component-base v0.25.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.25.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.25.0
	k8s.io/cri-api => k8s.io/cri-api v0.25.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.25.0
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.25.0
	k8s.io/kms => k8s.io/kms v0.25.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.25.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.25.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.25.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.25.0
	k8s.io/kubectl => k8s.io/kubectl v0.25.0
	k8s.io/kubelet => k8s.io/kubelet v0.25.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.25.0
	k8s.io/metrics => k8s.io/metrics v0.25.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.25.0
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.25.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.25.0
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v1.3.1
)
