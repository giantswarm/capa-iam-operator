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
	"github.com/giantswarm/capa-iam-controller/pkg/key"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/giantswarm/capa-iam-controller/pkg/awsclient"
	"github.com/giantswarm/capa-iam-controller/pkg/iam"
)

// AWSMachineTemplateReconciler reconciles a AWSMachineTemplate object
type AWSMachineTemplateReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
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
		return ctrl.Result{}, nil
	}

	clusterName := key.GetClusterIDFromLabels(awsMachineTemplate)

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
		return ctrl.Result{}, nil
	}

	var iamService *iam.IAMService
	{
		c := iam.IAMServiceConfig{
			AWSSession:  awsClientSession,
			ClusterName: clusterName,
			IAMRoleName: awsMachineTemplate.Spec.Template.Spec.IAMInstanceProfile,
			Log:         logger,
			RoleType:    iam.ControlPlaneRole,
		}
		iamService, err = iam.New(c)
		if err != nil {
			logger.Error(err, "Failed to generate IAM service")
			return ctrl.Result{}, err
		}
	}

	if awsMachineTemplate.DeletionTimestamp != nil {
		err = iamService.Delete()
		if err != nil {
			logger.Error(err, "failed to delete IAM Role")
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(awsMachineTemplate, key.CAPAIAMControllerFinalizer)
		err = r.Update(ctx, awsMachineTemplate)
		if err != nil {
			logger.Error(err, "failed to remove finalizer from AWSMachineTemplate")
			return ctrl.Result{}, err
		}

	} else {
		err = iamService.Reconcile()
		if err != nil {
			logger.Error(err, "failed to reconcile IAM Role")
			return ctrl.Result{}, err
		}

		controllerutil.AddFinalizer(awsMachineTemplate, key.CAPAIAMControllerFinalizer)
		err = r.Update(ctx, awsMachineTemplate)
		if err != nil {
			logger.Error(err, "failed to add finalizer on AWSMachineTemplate")
			return ctrl.Result{}, err
		}

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AWSMachineTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capa.AWSMachineTemplate{}).
		Complete(r)
}
