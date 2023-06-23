package key

import "github.com/giantswarm/microerror"

var clusterValuesConfigMapNotFound = &microerror.Error{
	Kind: "clusterValuesConfigMapNotFoundError",
}

var baseDomainNotFound = &microerror.Error{
	Kind: "baseDomainNotFoundError",
}
