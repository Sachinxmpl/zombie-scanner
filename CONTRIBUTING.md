# Contributing

Bug reports, false positives, and ideas for new checks are always welcome.

## The one rule: read-only

`zombie-scanner` must never make changes to anyone's AWS account. It only allows `Describe`, `Get`, and `List` calls. The only exception is `sts:AssumeRole`, which is needed for `--accounts`.

We enforce this in two ways:

- CI checks the code using `scripts/check-readonly.sh`.
- At runtime, `awsapi/readonly.go` blocks any other AWS call before it is sent.

Pull requests that add write operations will not be merged, even if they are behind a flag.

## Setting up

You need Go 1.26 or later and `golangci-lint` to run `make lint`.

```bash
git clone https://github.com/Sachinxmpl/zombie-scanner
cd zombie-scanner
make check
```

`make check` runs the same checks as CI: build, tests, lint, and read-only checks.

Tests use mock AWS data, so you don't need an AWS account.

## How the code is organized

| Folder | Purpose |
|---|---|
| `awsapi/` | The only package that talks to AWS |
| `collect/` | Collects AWS data into an `Inventory` |
| `detect/` | Contains the checks that find unused resources |
| `price/` | Handles monthly cost estimates |
| `filter/`, `render/` | Filters findings and formats reports |
| `zombie/` | Shared types |
| `test/` | Tests |

The `zombie`, `detect`, `price`, `filter`, and `render` packages must not use the AWS SDK.

`scripts/check-arch.sh` checks this rule.

## Adding a check

1. Add `detect/<name>.go` with a type that implements `Detector`. Register it in `init()`. See `detect/elb_idle.go` for a simple example.
2. List all required AWS permissions in `Needs()`.
3. If the check needs new data, add the collection logic in `collect/` and the corresponding method in `awsapi/`.
4. If the resource has a cost, add its price in `price/`.
5. Add tests in `test/`, including cases where an AWS call fails.
6. Run `make iam-policy` to update `docs/iam-policy.json`. CI will fail if this file is out of date.
7. Update the README's "What it checks" section, the false-positive issue template at `.github/ISSUE_TEMPLATE/false_positive.yml`, and `CHANGELOG.md`.

**When data is missing, report nothing.**

If a metric is missing, an AWS call fails, or tags cannot be read, the resource must be treated as unknown, not unused.

A false positive costs more trust than a missed finding.

## Sending a pull request

- Create a branch from `master`.
- Keep one change per pull request.
- Run `make check` before pushing.
- Use commit messages like `feat(detect): ...`, `fix(report): ...`, or `docs: ...`.

## Security problems

See [SECURITY.md](SECURITY.md) for how to report security issues. Please don't open a public issue.
