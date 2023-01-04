package controllers

import (
	"context"

	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	expcapa "sigs.k8s.io/cluster-api-provider-aws/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		if mt.DeletionTimestamp != nil && mt.Spec.Template.Spec.IAMInstanceProfile == roleName {
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
	for _, mt := range awsMachinePools.Items {
		if mt.DeletionTimestamp != nil && mt.Spec.AWSLaunchTemplate.IamInstanceProfile == roleName {
			return true, err
		}
	}

	return false, err
}
