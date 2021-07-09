[![CircleCI](https://circleci.com/gh/giantswarm/capa-iam-controller.svg?style=shield)](https://circleci.com/gh/giantswarm/capa-iam-controller)

# capa-iam-controller

CAPA-iam-controller is creating unique IAM roles for each CAPA cluster, it watches AWSMachineTemplate CRs and reads `AWSMachineTemplate.spec.template.spec.iamInstanceProfile` for ControlPlane and AWSMachinePool CRs and reads `AWSMachinePool.spec.awsLaunchTemplate.iamInstanceProfile`.

If the IAM role in CR is found in the AWS API it will skip the creation, if its missing it will create a new one from a template.

### IAM roles for Control Plane
 In addition to the IAM role for Control plane nodes, `capa-iam-controller` wil also create IAM role for `kiam` app and Route53 role for `external-dns` app.

You can disable creating KIAM and Route53 roles via arguments `--enable-kiam-role=false` and `--enable-route53-role=false`. Route53 role will be only created if KIAm role is enabled, as it depends on it.


### IAM roles for Worker nodes
For each `AWSMachinePool` CR, a separate IAM role will be created.
