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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/giantswarm/capa-iam-controller/pkg/awsclient"
	"github.com/giantswarm/capa-iam-controller/pkg/iam"
	"github.com/giantswarm/capa-iam-controller/pkg/key"
)

// AWSMachineTemplateReconciler reconciles a AWSMachineTemplate object
type AWSMachineTemplateReconciler struct {
	client.Client
	EnableKiamRole    bool
	EnableRoute53Role bool
	Log               logr.Logger
	Scheme            *runtime.Scheme
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsmachinetemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsmachinetemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsmachinetemplates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *AWSMachineTemplateReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var err error
	ctx := context.TODO()
	logger := r.Log.WithValues("namespace", req.Namespace, "awsMachineTemplate", req.Name)

	awsMachineTemplate := &capa.AWSMachineTemplate{}
	if err := r.Get(ctx, req.NamespacedName, awsMachineTemplate); err != nil {
		logger.Error(err, "AWSMachineTemplate does not exist")
		return ctrl.Result{}, err
	}

	clusterName := key.GetClusterIDFromLabels(awsMachineTemplate.ObjectMeta)

	logger = logger.WithValues("cluster", clusterName)

	var awsClientGetter *awsclient.AwsClient
	{
		c := awsclient.AWSClientConfig{
			ClusterName: clusterName,
			CtrlClient:  r.Client,
			Log:         logger,
		}
		awsClientGetter, err = awsclient.New(c)
		if err != nil {
			logger.Error(err, "failed to generate awsClientGetter")
			return ctrl.Result{}, err
		}
	}

	awsClientSession, err := awsClientGetter.GetAWSClientSession(ctx)
	if err != nil {
		logger.Error(err, "Failed to get aws client session")
		return ctrl.Result{}, err
	}

	var iamService *iam.IAMService
	{
		c := iam.IAMServiceConfig{
			AWSSession:   awsClientSession,
			ClusterName:  clusterName,
			MainRoleName: awsMachineTemplate.Spec.Template.Spec.IAMInstanceProfile,
			Log:          logger,
			RoleType:     iam.ControlPlaneRole,
		}
		iamService, err = iam.New(c)
		if err != nil {
			logger.Error(err, "Failed to generate IAM service")
			return ctrl.Result{}, err
		}
	}

	if awsMachineTemplate.DeletionTimestamp != nil {
		err = iamService.DeleteRole()
		if err != nil {
			return ctrl.Result{}, err
		}
		if r.EnableKiamRole {
			err = iamService.DeleteKiamRole()
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		// route53 role depends on KIAM role, so it will be crated only if both roles are enabled
		if r.EnableKiamRole && r.EnableRoute53Role {
			err = iamService.DeleteRoute53Role()
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		// remove finalizer from AWSCluster
		{
			awsCluster, err := key.GetAWSClusterByName(ctx, r.Client, clusterName)
			if err != nil {
				logger.Error(err, "failed to get awsCluster")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(awsCluster, key.FinalizerName(iam.ControlPlaneRole))
			err = r.Update(ctx, awsCluster)
			if err != nil {
				logger.Error(err, "failed to remove finalizer on AWSCluster")
				return ctrl.Result{}, err
			}
		}

		// remove finalizer from AWSMachineTemplate
		controllerutil.RemoveFinalizer(awsMachineTemplate, key.FinalizerName(iam.ControlPlaneRole))
		err = r.Update(ctx, awsMachineTemplate)
		if err != nil {
			logger.Error(err, "failed to remove finalizer from AWSMachineTemplate")
			return ctrl.Result{}, err
		}
	} else {
		err = iamService.ReconcileRole()
		if err != nil {
			return ctrl.Result{}, err
		}
		if r.EnableKiamRole {
			err = iamService.ReconcileKiamRole()
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		// route53 role depends on KIAM role
		if r.EnableKiamRole && r.EnableRoute53Role {
			err = iamService.ReconcileRoute53Role()
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		// add finalizer to AWSMachineTemplate
		controllerutil.AddFinalizer(awsMachineTemplate, key.FinalizerName(iam.ControlPlaneRole))
		err = r.Update(ctx, awsMachineTemplate)
		if err != nil {
			logger.Error(err, "failed to add finalizer on AWSMachineTemplate")
			return ctrl.Result{}, err
		}

		// add finalizer to AWSCluster
		{
			awsCluster, err := key.GetAWSClusterByName(ctx, r.Client, clusterName)
			if err != nil {
				logger.Error(err, "failed to get awsCluster")
				return ctrl.Result{}, err
			}
			controllerutil.AddFinalizer(awsCluster, key.FinalizerName(iam.ControlPlaneRole))
			err = r.Update(ctx, awsCluster)
			if err != nil {
				logger.Error(err, "failed to add finalizer on AWSCluster")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: time.Minute * 5,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AWSMachineTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capa.AWSMachineTemplate{}).
		Complete(r)
}
