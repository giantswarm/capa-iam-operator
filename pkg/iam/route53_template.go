package iam

const trustIdentityPolicyIRSA = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:{{.AWSDomain}}:iam::{{.AccountID}}:oidc-provider/{{.CloudFrontDomain}}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "{{.CloudFrontDomain}}:sub": "system:serviceaccount:{{.Namespace}}:{{.ServiceAccount}}"
        }
      }
    }{{if .AdditionalCloudFrontDomain}},
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:{{.AWSDomain}}:iam::{{.AccountID}}:oidc-provider/{{.AdditionalCloudFrontDomain}}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "{{.AdditionalCloudFrontDomain}}:sub": "system:serviceaccount:{{.Namespace}}:{{.ServiceAccount}}"
        }
      }
    }
    {{end}}
  ]
}
`

const albControllerTrustIdentityPolicyIRSA = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:{{.AWSDomain}}:iam::{{.AccountID}}:oidc-provider/{{.CloudFrontDomain}}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringLike": {
          "{{.CloudFrontDomain}}:sub": "system:serviceaccount:*:{{.ServiceAccount}}"
        }
      }
    }{{if .AdditionalCloudFrontDomain}},
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:{{.AWSDomain}}:iam::{{.AccountID}}:oidc-provider/{{.AdditionalCloudFrontDomain}}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringLike": {
          "{{.AdditionalCloudFrontDomain}}:sub": "system:serviceaccount:*:{{.ServiceAccount}}"
        }
      }
    }
    {{end}}
  ]
}
`

const externalDnsTrustIdentityPolicyIRSA = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:{{.AWSDomain}}:iam::{{.AccountID}}:oidc-provider/{{.CloudFrontDomain}}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringLike": {
          "{{.CloudFrontDomain}}:sub": "system:serviceaccount:*:*{{.ServiceAccount}}*"
        }
      }
    }{{if .AdditionalCloudFrontDomain}},
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:{{.AWSDomain}}:iam::{{.AccountID}}:oidc-provider/{{.AdditionalCloudFrontDomain}}"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringLike": {
          "{{.AdditionalCloudFrontDomain}}:sub": "system:serviceaccount:*:*{{.ServiceAccount}}*"
        }
      }
    }
    {{end}}
  ]
}
`

const route53RolePolicyTemplate = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "route53:ChangeResourceRecordSets",
      "Resource": [
        "arn:*:route53:::hostedzone/*"
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
}
`

const route53RolePolicyTemplateForCertManager = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "route53:GetChange",
      "Resource": "arn:*:route53:::change/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "route53:ChangeResourceRecordSets",
        "route53:ListResourceRecordSets"
      ],
      "Resource": "arn:*:route53:::hostedzone/*"
    },
    {
      "Effect": "Allow",
      "Action": "route53:ListHostedZonesByName",
      "Resource": "*"
    }
  ]
}
`
