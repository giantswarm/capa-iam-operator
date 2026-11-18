package controllers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/capa-iam-operator/v3/controllers"
	"github.com/giantswarm/capa-iam-operator/v3/pkg/iam"
	"github.com/giantswarm/capa-iam-operator/v3/pkg/key"
)

var _ = Describe("Finalizers", func() {
	const otherFinalizer = "other-operator.finalizers.giantswarm.io/test"

	var (
		ctx        context.Context
		namespace  string
		awsCluster *capa.AWSCluster
	)

	SetupNamespaceBeforeAfterEach(&namespace)

	// createAWSCluster creates an AWSCluster with the given finalizers and
	// returns the copy read back from the API.
	createAWSCluster := func(finalizers ...string) *capa.AWSCluster {
		cluster := &capa.AWSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "finalizer-test",
				Namespace:  namespace,
				Finalizers: finalizers,
			},
		}
		Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
		return cluster
	}

	// removeOtherFinalizer removes otherFinalizer from the current object in
	// the API, leaving awsCluster outdated.
	removeOtherFinalizer := func() {
		latest := &capa.AWSCluster{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(awsCluster), latest)).To(Succeed())
		var finalizers []string
		for _, f := range latest.Finalizers {
			if f != otherFinalizer {
				finalizers = append(finalizers, f)
			}
		}
		latest.Finalizers = finalizers
		Expect(k8sClient.Update(ctx, latest)).To(Succeed())
	}

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("AddFinalizer", func() {
		When("another controller removed its finalizer after the object was read", func() {
			BeforeEach(func() {
				awsCluster = createAWSCluster(otherFinalizer)
				removeOtherFinalizer()
			})

			It("returns a conflict and does not add the other finalizer back", func() {
				err := controllers.AddFinalizer(ctx, k8sClient, awsCluster, iam.ControlPlaneRole)
				Expect(k8serrors.IsConflict(err)).To(BeTrue())

				actual := &capa.AWSCluster{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(awsCluster), actual)).To(Succeed())
				Expect(actual.Finalizers).To(BeEmpty())
			})
		})
	})

	Describe("RemoveFinalizer", func() {
		When("another controller removed its finalizer after the object was read", func() {
			BeforeEach(func() {
				awsCluster = createAWSCluster(key.FinalizerName(iam.ControlPlaneRole), otherFinalizer)
				removeOtherFinalizer()
			})

			It("retries with the current object and does not add the other finalizer back", func() {
				err := controllers.RemoveFinalizer(ctx, k8sClient, awsCluster, iam.ControlPlaneRole)
				Expect(err).NotTo(HaveOccurred())

				actual := &capa.AWSCluster{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(awsCluster), actual)).To(Succeed())
				Expect(actual.Finalizers).To(BeEmpty())
			})
		})

		When("the object is being deleted and another controller removed its finalizer after the object was read", func() {
			BeforeEach(func() {
				awsCluster = createAWSCluster(key.FinalizerName(iam.ControlPlaneRole), otherFinalizer)
				Expect(k8sClient.Delete(ctx, awsCluster)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(awsCluster), awsCluster)).To(Succeed())
				removeOtherFinalizer()
			})

			It("retries with the current object and the object is deleted", func() {
				err := controllers.RemoveFinalizer(ctx, k8sClient, awsCluster, iam.ControlPlaneRole)
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Get(ctx, client.ObjectKeyFromObject(awsCluster), &capa.AWSCluster{})
				Expect(k8serrors.IsNotFound(err)).To(BeTrue())
			})
		})
	})
})
