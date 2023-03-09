package controllers_test

type RoleInfo struct {
	ExpectedName                     string
	ExpectedAssumeRolePolicyDocument string
	ExpectedPolicyName               string
	ExpectedPolicyDocument           string
	ReturnRoleArn                    string
}

const fakeRegion = "fakeregion"

var certManagerRoleInfo = RoleInfo{
	ExpectedName: "test-cluster-CertManager-Role",

	ExpectedAssumeRolePolicyDocument: `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::999666333:role/test-cluster-IAMManager-Role"
      },
      "Action": "sts:AssumeRole"
    },
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::123456789999:oidc-provider/foobar.cloudfront.net"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "foobar.cloudfront.net:sub": "system:serviceaccount:kube-system:cert-manager-controller"
        }
      }
    }
  ]
}
`,

	ExpectedPolicyName: "control-plane-test-cluster-policy",
	ExpectedPolicyDocument: `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "route53:GetChange",
      "Resource": "arn:aws:route53:::change/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "route53:ChangeResourceRecordSets",
        "route53:ListResourceRecordSets"
      ],
      "Resource": "arn:aws:route53:::hostedzone/*"
    },
    {
      "Effect": "Allow",
      "Action": "route53:ListHostedZonesByName",
      "Resource": "*"
    }
  ]
}
`,

	ReturnRoleArn: "arn:aws:iam::121245456767:role/test-cluster-CertManager-Role",
}

var externalDnsRoleInfo = RoleInfo{
	ExpectedName: "test-cluster-Route53Manager-Role",

	ExpectedAssumeRolePolicyDocument: `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::999666333:role/test-cluster-IAMManager-Role"
      },
      "Action": "sts:AssumeRole"
    },
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::123456789999:oidc-provider/foobar.cloudfront.net"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "foobar.cloudfront.net:sub": "system:serviceaccount:kube-system:external-dns"
        }
      }
    }
  ]
}
`,

	ExpectedPolicyName: "control-plane-test-cluster-policy",
	ExpectedPolicyDocument: `{
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
}
`,

	ReturnRoleArn: "arn:aws:iam::55554444:role/test-cluster-Route53Manager-Role",
}
