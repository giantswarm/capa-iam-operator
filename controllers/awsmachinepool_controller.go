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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	expcapa "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/giantswarm/capa-iam-operator/pkg/awsclient"
	"github.com/giantswarm/capa-iam-operator/pkg/iam"
	"github.com/giantswarm/capa-iam-operator/pkg/key"
)

// AWSMachinePoolReconciler reconciles a AWSMachinePool object
type AWSMachinePoolReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsmachinepools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsmachinepools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsmachinepools/finalizers,verbs=update

func (r *AWSMachinePoolReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var err error
	ctx := context.TODO()
	logger := r.Log.WithValues("namespace", req.Namespace, "awsMachinePool", req.Name)

	awsMachinePool := &expcapa.AWSMachinePool{}
	if err := r.Get(ctx, req.NamespacedName, awsMachinePool); err != nil {
		logger.Error(err, "AWSMachinePool does not exist")
		return ctrl.Result{}, err
	}
	// check if CR got CAPI watch-filter label
	if !key.HasCapiWatchLabel(awsMachinePool.Labels) {
		logger.Info(fmt.Sprintf("AWSMachinePool do not have %s=%s label, ignoring CR", key.ClusterWatchFilterLabel, "capi"))
		// ignoring this CR
		return ctrl.Result{}, nil
	}

	clusterName := key.GetClusterIDFromLabels(awsMachinePool.ObjectMeta)

	logger = logger.WithValues("cluster", clusterName)

	if awsMachinePool.Spec.AWSLaunchTemplate.IamInstanceProfile == "" {
		logger.Info("AWSMachinePool has empty .Spec.AWSLaunchTemplate.IamInstanceProfile, not creating IAM role")
		return ctrl.Result{}, nil
	}

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
			MainRoleName: awsMachinePool.Spec.AWSLaunchTemplate.IamInstanceProfile,
			Log:          logger,
			RoleType:     iam.NodesRole,
		}
		iamService, err = iam.New(c)
		if err != nil {
			logger.Error(err, "Failed to generate IAM service")
			return ctrl.Result{}, err
		}
	}

	if awsMachinePool.DeletionTimestamp != nil {
		err = iamService.DeleteRole()
		if err != nil {
			return ctrl.Result{}, err
		}

		// remove finalizer from AWSCluster
		{
			awsCluster, err := key.GetAWSClusterByName(ctx, r.Client, clusterName)
			if err != nil {
				logger.Error(err, "failed to get awsCluster")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(awsCluster, key.FinalizerName(iam.NodesRole))
			err = r.Update(ctx, awsCluster)
			if err != nil {
				logger.Error(err, "failed to remove finalizer on AWSCluster")
				return ctrl.Result{}, err
			}
		}

		// remove finalizer from AWSMachinePool
		controllerutil.RemoveFinalizer(awsMachinePool, key.FinalizerName(iam.NodesRole))
		err = r.Update(ctx, awsMachinePool)
		if err != nil {
			logger.Error(err, "failed to remove finalizer from AWSMachinePool")
			return ctrl.Result{}, err
		}
	} else {
		err = iamService.ReconcileRole()
		if err != nil {
			return ctrl.Result{}, err
		}

		// add finalizer to AWSMachinePool
		controllerutil.AddFinalizer(awsMachinePool, key.FinalizerName(iam.NodesRole))
		err = r.Update(ctx, awsMachinePool)
		if err != nil {
			logger.Error(err, "failed to add finalizer on AWSMachinePool")
			return ctrl.Result{}, err
		}

		// add finalizer to AWSCluster
		{
			awsCluster, err := key.GetAWSClusterByName(ctx, r.Client, clusterName)
			if err != nil {
				logger.Error(err, "failed to get awsCluster")
				return ctrl.Result{}, err
			}
			controllerutil.AddFinalizer(awsCluster, key.FinalizerName(iam.NodesRole))
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
func (r *AWSMachinePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&expcapa.AWSMachinePool{}).
		Complete(r)
}
