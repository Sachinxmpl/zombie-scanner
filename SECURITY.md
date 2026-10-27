# Security Policy

## Reporting a problem

Please report security problems privately through
[GitHub Security Advisories](https://github.com/Sachinxmpl/zombie-scanner/security/advisories/new),
not in a public issue.

You'll get a reply within a few days. If the problem is real, the fix goes into
the next release.

## What the tool can and can't do

zombie-scanner only reads. It never creates, changes or deletes anything in
your AWS account. It talks only to AWS, and sends no data anywhere else.

This is checked in two places.

**In CI, on every change:**

- [`scripts/check-readonly.sh`](scripts/check-readonly.sh) fails the build if:
  - code outside `awsapi/` creates an AWS client,
  - any AWS call the tool declares isn't a `Describe`, `Get` or `List`,
  - a known write call (like `DeleteVolume`) appears in the code,
  - the run-time guard below is removed.
- [`scripts/check-arch.sh`](scripts/check-arch.sh) fails the build if the
  checking or pricing code starts using the AWS SDK.

**While the tool runs:** every AWS client has a guard
([`awsapi/readonly.go`](awsapi/readonly.go)) that refuses any call that isn't a
`Describe`, `Get` or `List`, before anything is sent. The one exception is
`sts:AssumeRole`, which `--accounts` uses to log in to your other accounts. It
creates nothing.

Every AWS call the tool can make is listed in [`awsapi/api.go`](awsapi/api.go),
one interface per AWS service. That file is the full list: nothing else can
reach AWS.

## Credentials

The tool never reads, saves or sends your credentials itself. It uses the
normal AWS credential chain (environment variables, `~/.aws` files, SSO,
container and instance roles), and keeps credentials in memory only while it
runs.

`zombie-scanner iam-policy` prints the exact permissions it needs. None of them
allow writing.

## Checking a download

Every release comes with checksums, an SBOM (a list of everything built in) and
a signed record of how it was built:

```bash
sha256sum -c checksums.txt --ignore-missing

gh attestation verify zombie-scanner_0.2.0_linux_amd64.tar.gz \
  --repo Sachinxmpl/zombie-scanner
```

The signed record proves the file was built by this repository's release
workflow, from the tagged commit. If `gh attestation verify` fails or names a
different workflow, the file did not come from this project.

## Supported versions

Before 1.0, only the latest release gets security fixes. Don't use `v0.1.0`; it
was withdrawn.

Releases are built with the Go version in `go.mod`, and CI runs `govulncheck` on
every pull request to look for known problems in Go and every dependency.
