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
	"regexp"
	"strings"

	awsclientgo "github.com/aws/aws-sdk-go/aws/client"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/go-logr/logr"

	"github.com/giantswarm/capa-iam-operator/pkg/awsclient"
	"github.com/giantswarm/capa-iam-operator/pkg/iam"
	"github.com/giantswarm/capa-iam-operator/pkg/key"
)

const (
	IRSASecretSuffix = "irsa-cloudfront"
)

// AWSClusterReconciler reconciles a Secret object
type AWSClusterReconciler struct {
	client.Client
	EnableIRSARole            bool
	Log                       logr.Logger
	IAMClientAndRegionFactory func(awsclientgo.ConfigProvider) (iamiface.IAMAPI, string)
	AWSClient                 awsclient.AwsClientInterface
}

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update

func (r *AWSClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("namespace", req.Namespace, "AWSCluster", req.Name)
	logger.Info("Reconciling IRSA roles")

	awsCluster := &capa.AWSCluster{}
	err := r.Get(ctx, req.NamespacedName, awsCluster)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	if awsCluster.DeletionTimestamp != nil {
		return r.reconcileDelete(ctx, logger, awsCluster)
	}

	return r.reconcileNormal(ctx, logger, awsCluster)
}

func (r *AWSClusterReconciler) reconcileNormal(ctx context.Context, logger logr.Logger, awsCluster *capa.AWSCluster) (ctrl.Result, error) {
	logger.Info("reconcile normal")
	// add finalizer to AWSCluster
	if !controllerutil.ContainsFinalizer(awsCluster, key.FinalizerName(iam.IRSARole)) {
		patchHelper, err := patch.NewHelper(awsCluster, r.Client)
		if err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.AddFinalizer(awsCluster, key.FinalizerName(iam.IRSARole))
		err = patchHelper.Patch(ctx, awsCluster)
		if err != nil {
			logger.Error(err, "failed to add finalizer on AWSCluster")
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("successfully added finalizer to AWSCluster", "finalizer_name", key.FinalizerName(iam.IRSARole))
	}

	cm := &corev1.ConfigMap{}
	err := r.Get(
		ctx,
		types.NamespacedName{
			Namespace: awsCluster.Namespace,
			Name:      fmt.Sprintf("%s-%s", awsCluster.Name, "cluster-values"),
		},
		cm)
	if err != nil {
		logger.Error(err, "Failed to get the cluster-values configmap for cluster")
		return ctrl.Result{}, errors.WithStack(err)
	}

	if !controllerutil.ContainsFinalizer(cm, key.FinalizerName(iam.IRSARole)) {
		patchHelper, err := patch.NewHelper(cm, r.Client)
		if err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}
		controllerutil.AddFinalizer(cm, key.FinalizerName(iam.IRSARole))
		err = patchHelper.Patch(ctx, cm)
		if err != nil {
			logger.Error(err, "failed to add finalizer to configmap", "configmap", fmt.Sprintf("%s-%s", awsCluster.Name, "cluster-values"))
			return ctrl.Result{}, errors.WithStack(err)
		}
		logger.Info("successfully added finalizer to configmap", "finalizer_name", iam.IRSARole, "configmap", fmt.Sprintf("%s-%s", awsCluster.Name, "cluster-values"))
	}

	awsClusterRoleIdentity, err := key.GetAWSClusterRoleIdentity(ctx, r.Client, awsCluster.Spec.IdentityRef.Name)
	if err != nil {
		logger.Error(err, "could not get AWSClusterRoleIdentity")
		return ctrl.Result{}, errors.WithStack(err)
	}

	accountID, err := getAWSAccountID(awsClusterRoleIdentity)
	if err != nil {
		logger.Error(err, "Could not get account ID")
		return ctrl.Result{}, errors.WithStack(err)
	}

	baseDomain, err := key.GetBaseDomain(ctx, r.Client, awsCluster.Name, awsCluster.Namespace)
	if err != nil {
		logger.Error(err, "Could not get base domain")
		return ctrl.Result{}, errors.WithStack(err)
	}

	cloudFrontDomain := key.CloudFrontAlias(baseDomain)

	awsClientSession, err := r.AWSClient.GetAWSClientSession(ctx, awsCluster.Name, awsCluster.GetNamespace())
	if err != nil {
		logger.Error(err, "Failed to get aws client session", "cluster_name", awsCluster)
		return ctrl.Result{}, errors.WithStack(err)
	}

	var iamService *iam.IAMService
	{
		c := iam.IAMServiceConfig{
			AWSSession:                awsClientSession,
			ClusterName:               awsCluster.Name,
			MainRoleName:              "-",
			RoleType:                  iam.IRSARole,
			Log:                       logger,
			IAMClientAndRegionFactory: r.IAMClientAndRegionFactory,
		}
		iamService, err = iam.New(c)
		if err != nil {
			logger.Error(err, "Failed to generate IAM service")
		}
	}

	err = iamService.ReconcileRolesForIRSA(accountID, cloudFrontDomain)
	if err != nil {
		logger.Error(err, "Unable to reconcile role")
		return ctrl.Result{}, errors.WithStack(err)
	}

	return ctrl.Result{}, nil
}

func (r *AWSClusterReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, awsCluster *capa.AWSCluster) (ctrl.Result, error) {
	logger.Info("reconcile delete")
	awsClientSession, err := r.AWSClient.GetAWSClientSession(ctx, awsCluster.Name, awsCluster.Namespace)
	if err != nil {
		logger.Error(err, "Failed to get aws client session")
		return ctrl.Result{}, err
	}

	var iamService *iam.IAMService
	{
		c := iam.IAMServiceConfig{
			AWSSession:                awsClientSession,
			ClusterName:               awsCluster.Name,
			MainRoleName:              "-",
			RoleType:                  iam.IRSARole,
			Log:                       logger,
			IAMClientAndRegionFactory: r.IAMClientAndRegionFactory,
		}
		iamService, err = iam.New(c)
		if err != nil {
			logger.Error(err, "Failed to generate IAM service")
		}
	}

	err = iamService.DeleteRolesForIRSA()
	if err != nil {
		logger.Error(err, "Unable to reconcile role")
		return ctrl.Result{}, err
	}

	if controllerutil.ContainsFinalizer(awsCluster, key.FinalizerName(iam.IRSARole)) {
		patchHelper, err := patch.NewHelper(awsCluster, r.Client)
		if err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(awsCluster, key.FinalizerName(iam.IRSARole))
		err = patchHelper.Patch(ctx, awsCluster)
		if err != nil {
			logger.Error(err, "failed to remove finalizer from awsCluster", "finalizer_name", key.FinalizerName(iam.IRSARole), "cluster_name", awsCluster)
			return ctrl.Result{}, err
		}
		logger.Info("successfully removed finalizer from awsCluster", "finalizer_name", key.FinalizerName(iam.IRSARole), "cluster_name", awsCluster)
	}

	cm := &corev1.ConfigMap{}
	err = r.Get(
		ctx,
		types.NamespacedName{
			Namespace: awsCluster.Namespace,
			Name:      fmt.Sprintf("%s-%s", awsCluster.Name, "cluster-values"),
		},
		cm)
	if err != nil {
		logger.Error(err, "Failed to get the cluster-values configmap for cluster")
		return ctrl.Result{}, err
	}

	if controllerutil.ContainsFinalizer(cm, key.FinalizerName(iam.IRSARole)) {
		patchHelper, err := patch.NewHelper(cm, r.Client)
		if err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(cm, key.FinalizerName(iam.IRSARole))
		err = patchHelper.Patch(ctx, cm)
		if err != nil {
			logger.Error(err, "failed to remove finalizer from configmap")
			return ctrl.Result{}, err
		}
		logger.Info("successfully removed finalizer from configmap", "finalizer_name", iam.IRSARole)
	}
	return ctrl.Result{}, nil
}

func getAWSAccountID(awsClusterRoleIdentity *capa.AWSClusterRoleIdentity) (string, error) {
	arn := awsClusterRoleIdentity.Spec.RoleArn
	if arn == "" || len(strings.TrimSpace(arn)) < 1 {
		err := fmt.Errorf("unable to extract ARN from AWSClusterRoleIdentity %s", awsClusterRoleIdentity.Name)
		return "", err
	}

	re := regexp.MustCompile(`[-]?\d[\d,]*[\.]?[\d{2}]*`)
	accountID := re.FindAllString(arn, 1)[0]

	if accountID == "" || len(strings.TrimSpace(accountID)) < 1 {
		err := fmt.Errorf("unable to extract AWS account ID from ARN %s", arn)
		return "", err
	}

	return accountID, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AWSClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capa.AWSCluster{}).
		Complete(r)
}
