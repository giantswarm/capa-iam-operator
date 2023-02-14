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

const route53TrustIdentityPolicyWithIRSA = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "{{.KIAMRoleARN}}"
      },
      "Action": "sts:AssumeRole"
    },
    {
        "Effect": "Allow",
        "Principal": {
            "Federated": "arn:aws:iam::{{.AccountID}}:oidc-provider/{{.CloudFrontDomain}}"
        },
        "Action": "sts:AssumeRoleWithWebIdentity",
        "Condition": {
            "StringEquals": {
                "{{.CloudFrontDomain}}:sub": "system:serviceaccount:{{.Namespace}}:{{.ServiceAccount}}"
            }
        }
    }
  ]
}`

const route53RolePolicyTemplate = `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": "route53:GetChange",
            "Resource": "arn:aws:route53:::change/*",
            "Effect": "Allow"
        },
        {
            "Action": [
              "route53:ChangeResourceRecordSets",
              "route53:ListResourceRecordSets"
            ],
            "Resource": "*",
            "Effect": "Allow"
        },
        {
            "Action": [
                "route53:ListHostedZones",
                "route53:ListResourceRecordSets",
                "route53:ListHostedZonesByName"
            ],
            "Resource": "*",
            "Effect": "Allow"
        }
    ]
}`
