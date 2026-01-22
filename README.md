[![CircleCI](https://circleci.com/gh/giantswarm/capa-iam-operator.svg?style=shield)](https://circleci.com/gh/giantswarm/capa-iam-operator)

# capa-iam-operator

capa-iam-operator is creating unique IAM roles for each CAPA cluster, it watches AWSMachineTemplate CRs and reads `AWSMachineTemplate.spec.template.spec.iamInstanceProfile` for ControlPlane and AWSMachinePool CRs and reads `AWSMachinePool.spec.awsLaunchTemplate.iamInstanceProfile`.

If the IAM role in CR is found in the AWS API it will skip the creation, if its missing it will create a new one from a template.

### IAM roles for Control Plane
In addition to the IAM role for Control plane nodes, `capa-iam-operator` will also create Route53 role for `external-dns` app and other IRSA roles.

You can disable creating Route53 role via argument `--enable-route53-role=false`.


### IAM roles for Worker nodes
For each `AWSMachinePool` CR, a separate IAM role will be created.
