package iam

const kiamTrustIdentityPolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "{{.ControlPlaneRoleARN}}"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}`

const kiamRolePolicyTemplate = `{
    "Version": "2012-10-17",
    "Statement": {
        "Action": "sts:AssumeRole",
        "Resource": "*",
        "Effect": "Allow"
    }
}`
