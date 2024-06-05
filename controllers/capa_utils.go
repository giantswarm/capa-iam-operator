package controllers

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	errutils "k8s.io/apimachinery/pkg/util/errors"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	expcapa "sigs.k8s.io/cluster-api-provider-aws/v2/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/capa-iam-operator/pkg/key"
)

const maxPatchAttempts = 5

func isRoleUsedElsewhere(ctx context.Context, ctrlClient client.Client, roleName string) (bool, error) {
	var err error

	var awsMachineTemplates capa.AWSMachineTemplateList
	err = ctrlClient.List(
		ctx,
		&awsMachineTemplates,
	)
	if err != nil {
		return false, err
	}
	for _, mt := range awsMachineTemplates.Items {
		if mt.DeletionTimestamp == nil && mt.Spec.Template.Spec.IAMInstanceProfile == roleName {
			return true, err
		}
	}

	var awsMachinePools expcapa.AWSMachinePoolList
	err = ctrlClient.List(
		ctx,
		&awsMachinePools,
	)
	if err != nil {
		return false, err
	}
	for _, mp := range awsMachinePools.Items {
		if mp.DeletionTimestamp == nil && mp.Spec.AWSLaunchTemplate.IamInstanceProfile == roleName {
			return true, err
		}
	}

	return false, err
}

func removeFinalizer(ctx context.Context, k8sClient client.Client, object client.Object, role string) error {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(object, key.FinalizerName(role)) {
		logger.Info("finalizer already removed")
		return nil
	}

	for i := 1; i <= maxPatchAttempts; i++ {
		patchHelper, err := patch.NewHelper(object, k8sClient)
		if err != nil {
			logger.Error(err, "failed to create patch helper")
			return errors.WithStack(err)
		}
		controllerutil.RemoveFinalizer(object, key.FinalizerName(role))
		err = patchHelper.Patch(ctx, object)

		// If another controller has removed its finalizer while we're
		// reconciling this will fail with "Forbidden: no new finalizers can be
		// added if the object is being deleted". The actual response code is
		// 422 Unprocessable entity, which maps to StatusReasonInvalid in the
		// k8serrors package. We have to get the cluster again with the now
		// removed finalizer(s) and try again.
		invalidErr := errutils.FilterOut(err, func(err error) bool {
			return !k8serrors.IsInvalid(err)
		})

		if invalidErr != nil && i < maxPatchAttempts {
			logger.Info(fmt.Sprintf("patching object failed, trying again: %s", err.Error()))
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(object), object); err != nil {
				return microerror.Mask(err)
			}
			continue
		}
		if err != nil {
			logger.Error(err, "failed to remove finalizers")
			return microerror.Mask(err)
		}

		logger.Info("successfully removed finalizer")
		return nil
	}

	return fmt.Errorf("failed to remove finalizer after %d retries", maxPatchAttempts)
}
