# Security Policy

## Reporting a vulnerability

Please report security issues privately through
[GitHub Security Advisories](https://github.com/Sachinxmpl/zombie-scanner/security/advisories/new)
rather than opening a public issue.

Expect an acknowledgement within a few days. If the issue is confirmed, a fix
will land in the next patch release with credit unless you prefer otherwise.

## Threat model

zombie-scanner runs with read-only AWS credentials and makes no outbound network
calls other than to AWS API endpoints. It sends no telemetry, does not phone
home, and writes nothing to your account.

That property is enforced in CI, not merely documented:

- **`scripts/check-readonly.sh`** fails the build if a package outside `awsapi/`
  constructs an AWS SDK client, if any method declared on the AWS interfaces is
  not a `Describe`/`Get`/`List`, or if a known mutating operation appears in the
  source.
- **`scripts/check-arch.sh`** fails the build if the detection or pricing
  packages gain a dependency on the AWS SDK at all.

Every AWS call the tool is capable of making is declared in four interfaces in
[`awsapi/api.go`](awsapi/api.go). That file is the complete attack surface, and
it is twelve declarations long.

## Credentials

zombie-scanner never reads, stores, or transmits credentials itself. It uses the
standard AWS SDK credential chain — environment, shared config, SSO, container
and instance roles — and holds the resolved credentials only in memory for the
duration of a scan.

Run `zombie-scanner iam-policy` for the minimal permission set. Nothing in it
grants write access.

## Release integrity

Release artifacts carry checksums, an SBOM, and a build provenance attestation:

```bash
sha256sum -c checksums.txt --ignore-missing

gh attestation verify zombie-scanner_0.1.1_linux_amd64.tar.gz \
  --repo Sachinxmpl/zombie-scanner
```

Attestation proves the artifact was built from a specific commit by this
repository's release workflow. The signed predicate is
[SLSA provenance v1](https://slsa.dev/provenance/v1), and the recorded builder
identity is this repository's release workflow at the tag being released:

```
https://github.com/Sachinxmpl/zombie-scanner/.github/workflows/release.yml@refs/tags/v0.1.1
```

If `gh attestation verify` reports a different builder, the artifact did not
come from this project.

## Supported versions

Pre-1.0. Security fixes land on the latest release only. `v0.1.0` was withdrawn
before general availability and should not be used; start at `v0.1.1`.

The release toolchain is pinned to the Go version declared in `go.mod`, and CI
runs `govulncheck` against the standard library and every dependency on each
pull request.
