[![CircleCI](https://circleci.com/gh/giantswarm/capa-iam-controller.svg?style=shield)](https://circleci.com/gh/giantswarm/capa-iam-controller)

# capa-iam-controller

CAPA-iam-controller is creating unique IAM roles for each CAPA cluster, it watches AWSMachineTemplate CRs and reads `AWSMachineTemplate.spec.template.spec.iamInstanceProfile` for ControlPlane and AWSMachinePool CRs and reads `AWSMachinePool.spec.awsLaunchTemplate.iamInstanceProfile`.

If the IAM role in CR is found in the AWS API it will skip the creation, if its missing it will create a new one from a template.
