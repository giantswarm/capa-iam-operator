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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

// SecretReconciler reconciles a Secret object
type SecretReconciler struct {
	client.Client
	EnableIRSARole            bool
	Log                       logr.Logger
	Scheme                    *runtime.Scheme
	IAMClientAndRegionFactory func(awsclientgo.ConfigProvider) (iamiface.IAMAPI, string)
}

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update

func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("namespace", req.Namespace, "secret", req.Name)

	if !r.EnableIRSARole {
		logger.Info("IRSA is not enabled")
		return ctrl.Result{}, nil
	}

	// We can say its a secret created by the IRSA operator if it has this suffix
	if !strings.HasSuffix(req.Name, IRSASecretSuffix) {
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling IRSA Secrets")

	secret := &corev1.Secret{}
	err := r.Get(ctx, req.NamespacedName, secret)
	if err != nil {
		logger.Error(err, "Failed to get the secret")
		return ctrl.Result{}, err
	}

	var result ctrl.Result

	if secret.DeletionTimestamp == nil {
		logger.Info("IRSA Secrets - reconcile normal")
		result, err = r.reconcileNormal(ctx, logger, secret)
	} else {
		logger.Info("IRSA Secrets - reconcile delete")
		result, err = r.reconcileDelete(ctx, logger, secret)
	}

	return result, err
}

func (r *SecretReconciler) reconcileNormal(ctx context.Context, logger logr.Logger, secret *corev1.Secret) (ctrl.Result, error) {
	// add finalizer to Secret
	if !controllerutil.ContainsFinalizer(secret, key.FinalizerName(iam.IRSARole)) {
		patchHelper, err := patch.NewHelper(secret, r.Client)
		if err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.AddFinalizer(secret, key.FinalizerName(iam.IRSARole))
		err = patchHelper.Patch(ctx, secret)
		if err != nil {
			logger.Error(err, "failed to add finalizer on Secret")
			return ctrl.Result{}, err
		}
		logger.Info("successfully added finalizer to Secret", "finalizer_name", key.FinalizerName(iam.IRSARole))
	}
	accountID, err := getAWSAccountID(secret)
	if err != nil {
		logger.Error(err, "Could not get account ID")
		return ctrl.Result{}, err
	}

	domain, err := getCloudFrontDomain(secret)
	if err != nil {
		logger.Error(err, "Could not get the cloudfront domain")
		return ctrl.Result{}, err
	}

	clusterName := strings.TrimSuffix(secret.Name, "-"+IRSASecretSuffix)

	var awsClientGetter *awsclient.AwsClient
	{
		c := awsclient.AWSClientConfig{
			ClusterName: clusterName,
			CtrlClient:  r.Client,
			Log:         logger,
		}
		awsClientGetter, err = awsclient.New(c)
		if err != nil {
			logger.Error(err, "Failed to generate awsClientGetter")
			return ctrl.Result{}, err
		}
	}

	awsClientSession, err := awsClientGetter.GetAWSClientSession(ctx, secret.GetNamespace())
	if err != nil {
		logger.Error(err, "Failed to get aws client session", "cluster_name", clusterName)
		return ctrl.Result{}, err
	}

	var iamService *iam.IAMService
	{
		c := iam.IAMServiceConfig{
			AWSSession:                awsClientSession,
			ClusterName:               clusterName,
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

	err = iamService.ReconcileRolesForIRSA(accountID, domain)
	if err != nil {
		logger.Error(err, "Unable to reconcile role")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SecretReconciler) reconcileDelete(ctx context.Context, logger logr.Logger, secret *corev1.Secret) (ctrl.Result, error) {
	var err error
	clusterName := strings.TrimSuffix(secret.Name, "-"+IRSASecretSuffix)

	var awsClientGetter *awsclient.AwsClient
	{
		c := awsclient.AWSClientConfig{
			ClusterName: clusterName,
			CtrlClient:  r.Client,
			Log:         logger,
		}
		awsClientGetter, err = awsclient.New(c)
		if err != nil {
			logger.Error(err, "Failed to generate awsClientGetter")
			return ctrl.Result{}, err
		}
	}

	awsClientSession, err := awsClientGetter.GetAWSClientSession(ctx, secret.GetNamespace())
	if err != nil {
		logger.Error(err, "Failed to get aws client session")
		return ctrl.Result{}, err
	}

	var iamService *iam.IAMService
	{
		c := iam.IAMServiceConfig{
			AWSSession:                awsClientSession,
			ClusterName:               clusterName,
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

	if controllerutil.ContainsFinalizer(secret, key.FinalizerName(iam.IRSARole)) {
		patchHelper, err := patch.NewHelper(secret, r.Client)
		if err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(secret, key.FinalizerName(iam.IRSARole))
		err = patchHelper.Patch(ctx, secret)
		if err != nil {
			logger.Error(err, "failed to remove finalizer on Secret")
			return ctrl.Result{}, err
		}
		logger.Info("successfully removed finalizer from Secret", "finalizer_name", key.FinalizerName(iam.IRSARole))
	}
	return ctrl.Result{}, nil
}

func getAWSAccountID(secret *corev1.Secret) (string, error) {
	data := secret.Data
	arn := string(data["arn"])

	if arn == "" || len(strings.TrimSpace(arn)) < 1 {
		err := fmt.Errorf("unable to extract ARN from secret %s", secret.Name)
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

func getCloudFrontDomain(secret *corev1.Secret) (string, error) {
	data := secret.Data
	domain := string(data["domain"])

	if domain == "" || len(strings.TrimSpace(domain)) < 1 {
		err := fmt.Errorf("unable to extract CloudFront domain from secret %s", secret.Name)
		return "", err
	}

	return domain, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(r)
}
