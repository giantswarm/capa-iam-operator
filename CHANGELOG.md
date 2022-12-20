# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Secrets reconciler for IRSA to support `external-dns`

## [0.4.4] - 2022-11-29

### Fixed

- Add `ec2:DescribeVolumesModifications` to the control-plane role so that resizing volumes work.

## [0.4.3] - 2022-11-24

### Fixed

- Check for other resources using the same IAM instance profile as the resource being deleted and skip deleting the IAM role if others found.

## [0.4.2] - 2022-11-01

### Changed

- `PodSecurityPolicy` are removed on newer k8s versions, so only apply it if object is registered in the k8s API.

### Added

- Tolerate running on control-plane nodes if workers unavailable

## [0.4.1] - 2022-07-13

## [0.4.0] - 2022-04-19

- Add VerticalPodAutoscaler CR.
- Add IAM role creation for bastion node.


## [0.3.2] - 2022-03-03

- Added to `aws-app-collection`.

## [0.3.0] - 2021-10-06

- Renamed from `capa-iam-controller` to `capa-iam-operator`

## [0.2.0] - 2021-07-23

- Restrict `secretmanager` service permissions to access secrets with CAPI prefix.
- Only watch for CRs with capi watch filter.
- AWSMachinteTemplate controller - only watch for CRs with control plane role.

## [0.1.1] - 2021-07-15

- Rename Route53 and KIAM role names to match previous naming scheme.

## [0.1.0] - 2021-07-14

- Implement `AWSMachineTemplate` reconciler.
- Implement `AWSMachinePool` reconciler.

[Unreleased]: https://github.com/giantswarm/capa-iam-operator/compare/v0.4.4...HEAD
[0.4.4]: https://github.com/giantswarm/capa-iam-operator/compare/v0.4.3...v0.4.4
[0.4.3]: https://github.com/giantswarm/capa-iam-operator/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/giantswarm/capa-iam-operator/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/giantswarm/capa-iam-operator/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/giantswarm/capa-iam-operator/compare/v0.3.2...v0.4.0
[0.3.2]: https://github.com/giantswarm/capa-iam-operator/compare/v0.3.0...v0.3.2
[0.3.0]: https://github.com/giantswarm/capa-iam-operator/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/giantswarm/capa-iam-operator/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/giantswarm/capa-iam-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/capa-iam-operator/compare/v1.0.0...v0.1.0
