package iam

import (
	"errors"

	awsiamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/giantswarm/microerror"
)

var invalidClusterError = &microerror.Error{
	Kind: "invalidClusterError",
}

func IsNotFound(err error) bool {
	var nsee *awsiamtypes.NoSuchEntityException
	return errors.As(err, &nsee)
}

func IsAlreadyExists(err error) bool {
	var eaee *awsiamtypes.EntityAlreadyExistsException
	return errors.As(err, &eaee)
}
