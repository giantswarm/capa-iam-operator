package iam

const route53TrustIdentityPolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "{{.KIAMRoleARN}}"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}`

const route53RolePolicyTemplate = `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": "route53:ChangeResourceRecordSets",
            "Resource": [
                "arn:aws:route53:::hostedzone/*"
            ],
            "Effect": "Allow"
        },
        {
            "Action": [
                "route53:ListHostedZones",
                "route53:ListResourceRecordSets"
            ],
            "Resource": "*",
            "Effect": "Allow"
        }
    ]
}`
