package controllers

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	expcapa "sigs.k8s.io/cluster-api-provider-aws/v2/exp/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/capa-iam-operator/v3/pkg/key"
)

const (
	GiantSwarmReleaseLabel = "release.giantswarm.io/version"
	maxPatchAttempts       = 5
)

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

// addFinalizer adds the role's finalizer with a merge patch that carries the
// object's resourceVersion. A merge patch replaces the whole finalizers list,
// so without the resourceVersion a patch built from an outdated object could
// add back finalizers that other controllers have removed in the meantime.
// With it, the API server rejects the patch with a 409 Conflict and the
// reconciliation is retried with the current object.
func addFinalizer(ctx context.Context, k8sClient client.Client, object client.Object, role string) error {
	if controllerutil.ContainsFinalizer(object, key.FinalizerName(role)) {
		return nil
	}

	original, ok := object.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("failed to copy object %q", client.ObjectKeyFromObject(object))
	}
	controllerutil.AddFinalizer(object, key.FinalizerName(role))

	return k8sClient.Patch(ctx, object, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{}))
}

func removeFinalizer(ctx context.Context, k8sClient client.Client, object client.Object, role string) error {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(object, key.FinalizerName(role)) {
		logger.Info("finalizer already removed")
		return nil
	}

	for i := 1; i <= maxPatchAttempts; i++ {
		original, ok := object.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("failed to copy object %q", client.ObjectKeyFromObject(object))
		}
		controllerutil.RemoveFinalizer(object, key.FinalizerName(role))
		// The patch carries the object's resourceVersion (see addFinalizer).
		err := k8sClient.Patch(ctx, object, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{}))

		// If another controller has changed the object since we read it, this
		// fails with 409 Conflict. If another controller has removed its
		// finalizer from an object that is being deleted, it can also fail
		// with "Forbidden: no new finalizers can be added if the object is
		// being deleted", a 422 Unprocessable entity, which maps to
		// StatusReasonInvalid in the k8serrors package. In both cases we have
		// to get the object again and try again.
		if (k8serrors.IsConflict(err) || k8serrors.IsInvalid(err)) && i < maxPatchAttempts {
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
