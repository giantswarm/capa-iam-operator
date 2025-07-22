/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"maps"

	awsclientgo "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/giantswarm/microerror"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cluster-api/controllers/external"
	expcapi "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/giantswarm/capa-iam-operator/pkg/awsclient"
	"github.com/giantswarm/capa-iam-operator/pkg/iam"
	"github.com/giantswarm/capa-iam-operator/pkg/key"
)

// MachinePoolReconciler reconciles a AWSMachinePool object
type MachinePoolReconciler struct {
	client.Client
	IAMClientFactory func(awsclientgo.ConfigProvider, string) iamiface.IAMAPI
	AWSClient        awsclient.AwsClientInterface
}

func (r *MachinePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	machinePool := &expcapi.MachinePool{}
	if err := r.Get(ctx, req.NamespacedName, machinePool); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.WithStack(err)
	}

	logger = logger.WithValues("cluster", machinePool.Spec.ClusterName)
	ctx = log.IntoContext(ctx, logger)

	cluster, err := util.GetClusterByName(ctx, r.Client, machinePool.Namespace, machinePool.Spec.ClusterName)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get cluster for machinepool")
	}

	infraMachinePool, err := external.Get(ctx, r.Client, &machinePool.Spec.Template.Spec.InfrastructureRef)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if machinePool.Spec.Template.Spec.InfrastructureRef.Kind != "AWSMachinePool" && machinePool.Spec.Template.Spec.InfrastructureRef.Kind != "KarpenterMachinePool" {
		logger.Info("we only care about AWSMachinePool or KarpenterMachinePool, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// We just switched the type that we are adding the finalizer to.
	// We need to remove the finalizers from existing MachinePools that may have the finalizer.
	// This bit can be removed on the next release of the operator, after all `MachinePools` have been reconciled
	// and no longer contain the finalizer.
	err = removeFinalizer(ctx, r.Client, machinePool, iam.NodesRole)
	if err != nil {
		logger.Error(err, "failed to remove finalizer from MachinePool")
		return ctrl.Result{}, errors.WithStack(err)
	}

	var iamInstanceProfile string
	var found bool

	// The infra machine pool may be a AWSMachinePool or KarpenterMachinePool. They store the iamInstanceProfile in different fields.
	iamInstanceProfile, found, err = unstructured.NestedString(infraMachinePool.Object, "spec", "awsLaunchTemplate", "iamInstanceProfile")
	if err != nil {
		logger.Error(err, "error retrieving iamInstanceProfile", "infraMachinePool", machinePool.Spec.Template.Spec.InfrastructureRef.Name)
		return ctrl.Result{}, errors.New("failed to get iamInstanceProfile")
	}
	if !found {
		// If we don't find it, let's try the `KarpenterMachinePool` field instead.
		iamInstanceProfile, found, err = unstructured.NestedString(infraMachinePool.Object, "spec", "ec2NodeClass", "instanceProfile")
		if err != nil {
			logger.Error(err, "error retrieving .spec.ec2NodeClass.instanceProfile", "infraMachinePool", machinePool.Spec.Template.Spec.InfrastructureRef.Name)
			return ctrl.Result{}, errors.New("failed to get iamInstanceProfile")
		}
		if !found {
			return ctrl.Result{}, errors.New("failed to get iamInstanceProfile")
		}
	}

	if iamInstanceProfile == "" {
		return ctrl.Result{}, errors.New("infra MachinePool has empty iamInstanceProfile, not reconciling IAM role")
	}

	awsCluster, err := key.GetAWSClusterByName(ctx, r.Client, machinePool.Spec.ClusterName, req.Namespace)
	if err != nil {
		return ctrl.Result{}, microerror.Mask(err)
	}

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, awsCluster) {
		logger.Info("Reconciliation is paused for infra Cluster object referenced in the Cluster")
		return ctrl.Result{}, nil
	}

	awsClusterRoleIdentity, err := key.GetAWSClusterRoleIdentity(ctx, r.Client, awsCluster.Spec.IdentityRef.Name)
	if err != nil {
		logger.Error(err, "could not get AWSClusterRoleIdentity")
		return ctrl.Result{}, microerror.Mask(err)
	}

	awsClientSession, err := r.AWSClient.GetAWSClientSession(awsClusterRoleIdentity.Spec.RoleArn, awsCluster.Spec.Region)
	if err != nil {
		logger.Error(err, "Failed to get aws client session")
		return ctrl.Result{}, errors.WithStack(err)
	}

	var iamService *iam.IAMService
	{
		c := iam.IAMServiceConfig{
			ObjectLabels:     maps.Clone(infraMachinePool.GetLabels()),
			AWSSession:       awsClientSession,
			ClusterName:      cluster.Name,
			MainRoleName:     iamInstanceProfile,
			Log:              logger,
			RoleType:         iam.NodesRole,
			Region:           awsCluster.Spec.Region,
			IAMClientFactory: r.IAMClientFactory,
			CustomTags:       awsCluster.Spec.AdditionalTags,
		}
		iamService, err = iam.New(c)
		if err != nil {
			logger.Error(err, "Failed to generate IAM service")
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	if machinePool.DeletionTimestamp != nil {
		return r.reconcileDelete(ctx, infraMachinePool, iamService, iamInstanceProfile)
	}
	return r.reconcileNormal(ctx, infraMachinePool, iamService)
}

func (r *MachinePoolReconciler) reconcileDelete(ctx context.Context, infraMachinePool *unstructured.Unstructured, iamService *iam.IAMService, iamInstanceProfile string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	roleUsed, err := isRoleUsedElsewhere(ctx, r.Client, iamInstanceProfile)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	if !roleUsed {
		err = iamService.DeleteRole()
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	err = removeFinalizer(ctx, r.Client, infraMachinePool, iam.NodesRole)
	if err != nil {
		logger.Error(err, "failed to remove finalizer from infrastructure MachinePool")
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}

func (r *MachinePoolReconciler) reconcileNormal(ctx context.Context, infraMachinePool *unstructured.Unstructured, iamService *iam.IAMService) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(infraMachinePool, key.FinalizerName(iam.NodesRole)) {
		patchHelper, err := patch.NewHelper(infraMachinePool, r.Client)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		controllerutil.AddFinalizer(infraMachinePool, key.FinalizerName(iam.NodesRole))
		err = patchHelper.Patch(ctx, infraMachinePool)
		if err != nil {
			logger.Error(err, "failed to add finalizer on infrastructure MachinePool")
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("successfully added finalizer to infrastructure MachinePool", "finalizer_name", key.FinalizerName(iam.NodesRole))
	}

	err := iamService.ReconcileRole()
	if err != nil {
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MachinePoolReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&expcapi.MachinePool{}).
		Complete(r)
}
