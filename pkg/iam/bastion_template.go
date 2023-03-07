package iam

const bastionPolicyTemplate = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "s3:HeadBucket",
        "s3:HeadObject",
        "s3:GetBucket",
        "s3:GetObject",
        "s3:GetObjectAcl",
        "s3:GetObjectVersion"
      ],
      "Resource": [
        "arn:aws:s3:::*-capa-*"
      ],
      "Effect": "Allow"
    }
  ]
}
`
