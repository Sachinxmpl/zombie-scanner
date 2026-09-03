# zombie-scanner

**Find the AWS resources that are dead but still billing.**

A single-binary CLI that reads your AWS account and prints the resources nothing
is using — with an estimated monthly cost for each one, and a total at the
bottom.

It never creates, modifies, or deletes anything.

```console
$ zombie-scanner
Account 520646130605 · us-east-1 · scanned 2026-09-03 04:12 UTC

RESOURCE               TYPE         REGION      CONF    ~$/MO    REASON
nat-1fd0486ae1b9f4058  nat-gateway  us-east-1   MEDIUM  $32.85   104799 bytes out over 14 days (checked 2026-09-03)
vol-01aa332c9f5cd7872  ebs-volume   us-east-1   HIGH    $8.00    Unattached; created 45 days ago (100 GiB gp3)
snap-09ea129477b096c4e ebs-snapshot us-east-1   LOW     $5.00    Created 412 days ago (100 GiB), not referenced by any AMI
eipalloc-0c…d8323fb724 elastic-ip   us-east-1   HIGH    $3.65    Public IP 3.234.180.84 is allocated but associated with nothing

4 zombies · estimated zombie spend ~$49.50/month · figures are estimates
```

---

## Why

Every AWS resource bills one of two ways:

| Billing shape        | Meaning                                       | Cost when idle |
| -------------------- | --------------------------------------------- | -------------- |
| **Usage-billed**     | Charged for work done — Lambda calls, S3 GETs | ~$0            |
| **Existence-billed** | Charged for existing — EBS, NAT, Elastic IPs  | Full price     |

**A zombie is an existence-billed resource that nothing is using.** A volume
detached during a migration eighteen months ago. A NAT gateway in a VPC nobody
deploys to. An Elastic IP left over from an instance that was terminated.

None of them appear in a cost anomaly report, because nothing anomalous is
happening. They just bill, every hour, forever.

## Confidence, so you can trust the output

Two findings are not equally certain, and presenting them identically is how a
scanner loses its users. Every finding carries a level:

| Level      | Meaning                                     | Example                          |
| ---------- | ------------------------------------------- | -------------------------------- |
| **HIGH**   | Orphaned by definition — the API says so    | Unattached volume, idle EIP      |
| **MEDIUM** | Idle according to metrics over a window     | NAT with no bytes for 14 days    |
| **LOW**    | A heuristic that needs human judgment       | Snapshot older than 90 days      |

Two rules outrank every feature in this tool:

1. **A resource younger than the metric window is never judged.** No history, no verdict.
2. **Missing metric data is never evidence of idleness.** No data means *unknown*, not *idle*.

If CloudWatch returns nothing for a load balancer — a wrong dimension, a missing
permission, an outage — the tool stays quiet rather than reporting every load
balancer in your account as unused.

## Install

```bash
go install github.com/Sachinxmpl/zombie-scanner@latest
```

Or build from source:

```bash
git clone https://github.com/Sachinxmpl/zombie-scanner
cd zombie-scanner
make build
```

Requires Go 1.22 or later. Pre-built binaries and a Homebrew tap are coming with
the first tagged release.

## Usage

```bash
zombie-scanner                                  # scan your default region
zombie-scanner --region eu-west-1
zombie-scanner --all-regions                    # every region you have enabled
zombie-scanner --profile prod --all-regions
```

Filter the noise:

```bash
zombie-scanner --min-cost 10                    # only findings above $10/month
zombie-scanner --confidence high                # only what the API stated outright
zombie-scanner --only ebs-unattached,nat-idle   # pick your checks
zombie-scanner --skip snapshot-aged
```

Tune the thresholds:

```bash
zombie-scanner --snapshot-age-days 180 --stopped-days 60 --idle-window-days 30
```

Everything is also settable through the environment, which is handy in CI:

```bash
ZOMBIE_SCANNER_MIN_COST=25 zombie-scanner --all-regions
```

Precedence is `flag > environment > default`.

Run `zombie-scanner --help` for the full surface, or `zombie-scanner detectors`
to see what each check looks for.

## What it detects

| Detector             | Confidence | What it looks for                             | Cost basis              |
| -------------------- | ---------- | --------------------------------------------- | ----------------------- |
| `ebs-unattached`     | HIGH       | Volumes in state `available`                  | size × per-GiB rate     |
| `eip-unassociated`   | HIGH       | Elastic IPs attached to nothing               | flat monthly            |
| `instance-stopped`   | MEDIUM     | Stopped instances still paying for their disks | sum of attached volumes |
| `nat-idle`           | MEDIUM     | NAT gateways below an outbound-bytes floor    | hourly × 730            |
| `elb-idle`           | MEDIUM     | ALBs below a request-count floor              | hourly × 730            |
| `snapshot-aged`      | LOW        | Old snapshots that no AMI references          | size × snapshot rate    |

`instance-stopped` is the one people are surprised by. Stopping an instance
stops the compute charge — the attached EBS volumes keep billing at full price,
indefinitely. A stopped `t3.large` with a 500 GiB root volume is $50/month that
looks free in the console.

`snapshot-aged` cross-references every registered AMI first, because deleting a
snapshot an AMI depends on breaks the AMI. If that cross-reference cannot be
built, the check reports nothing rather than guessing.

## Use it in CI

```bash
zombie-scanner --all-regions --fail-if-above 100
```

| Exit code | Meaning                                            |
| --------- | -------------------------------------------------- |
| `0`       | Scan completed (zombies may exist)                 |
| `1`       | Fatal — no credentials, bad flags                  |
| `2`       | Scan succeeded, spend exceeded `--fail-if-above`   |

```yaml
- name: Check for zombie AWS spend
  run: zombie-scanner --all-regions --fail-if-above 100
```

## Machine-readable output

```bash
zombie-scanner --json | jq '.summary.total_monthly_usd'
zombie-scanner --json | jq -r '.findings[] | select(.confidence == "HIGH") | .resource_id'
```

```json
{
  "schema_version": "1",
  "account_id": "520646130605",
  "regions": ["us-east-1"],
  "findings": [
    {
      "resource_id": "vol-01aa332c9f5cd7872",
      "resource_type": "ebs-volume",
      "detector": "ebs-unattached",
      "confidence": "HIGH",
      "reason": "Unattached; created 45 days ago (100 GiB gp3)",
      "monthly_cost_usd": 8,
      "cost_basis": "100 GiB x $0.080/GiB-mo x 1.00 (us-east-1)"
    }
  ],
  "errors": [],
  "summary": { "total_monthly_usd": 8, "zombie_count": 1 }
}
```

Keys are `snake_case` and stable within a major version. `findings` and `errors`
are always arrays, never `null`. `errors` is included on purpose so a script can
tell "a clean account" from "the scan could not see anything".

## Permissions

```bash
zombie-scanner iam-policy
```

The policy is **generated from the code** — each check declares the API calls it
makes, and the policy is the union of them. It cannot drift from what the tool
actually does, and CI fails the build if the published copy falls out of date.

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "cloudwatch:GetMetricData",
      "ec2:DescribeAddresses",
      "ec2:DescribeImages",
      "ec2:DescribeInstances",
      "ec2:DescribeNatGateways",
      "ec2:DescribeRegions",
      "ec2:DescribeSnapshots",
      "ec2:DescribeVolumes",
      "elasticloadbalancing:DescribeLoadBalancers",
      "sts:GetCallerIdentity"
    ],
    "Resource": "*"
  }]
}
```

`Resource` is `"*"` because the EC2 `Describe*` actions do not support
resource-level permissions. AWS's managed `ReadOnlyAccess` also works if you
prefer a managed policy, though it grants considerably more than this.

Need less? `zombie-scanner iam-policy --only ebs-unattached` prints a policy for
just that check.

Credentials come from the standard AWS chain — environment, `~/.aws/config`,
SSO, instance roles. If `aws sts get-caller-identity` works, this works.

## Read-only, and provable

There is no `--delete` flag, and there will not be one.

> A scanner with a false positive is an annoyance. A deleter with a false
> positive is an outage.

The tool reports, you decide, your existing tooling executes. That boundary is
enforced rather than promised:

- Every AWS call the tool can make is declared in a handful of narrow Go
  interfaces. Every method on them is a `Describe` or a `Get`.
- CI fails the build if a mutating call appears anywhere in the codebase.
- CI fails the build if the detection or pricing packages gain a dependency on
  the AWS SDK at all.

Those are checks you can watch go green on every pull request, not claims you
have to take on faith.

## About the numbers

Costs are estimates against public list prices, and every figure is labelled as
one. They come from a small embedded table rather than the AWS Pricing API,
because a number a human is about to eyeball is better approximately right and
instant than precisely right and slow.

Your real bill is probably lower — enterprise discounts, savings plans and
private pricing all apply. Every finding shows its arithmetic so you can check
it:

```console
$ zombie-scanner -v
vol-01aa332c9f5cd7872  ebs-volume  us-east-1  HIGH  $8.00  Unattached; created 45 days ago (100 GiB gp3)
                       100 GiB x $0.080/GiB-mo x 1.00 (us-east-1)
```

Snapshot costs are upper bounds: snapshots bill incrementally, and the tool
prices them at full size and says so.

## How it compares

| Tool                                                | What it does                                | How this differs                                        |
| --------------------------------------------------- | ------------------------------------------- | ------------------------------------------------------- |
| [awsweeper](https://github.com/jckuester/awsweeper) | **Deletes** resources matching a YAML filter | It executes an intent you already have; this supplies the intent |
| [aws-nuke](https://github.com/rebuy-de/aws-nuke)    | Deletes everything in an account            | Teardown, not discovery                                 |
| [cloud-custodian](https://cloudcustodian.io)        | Policy engine — you write the rules         | This ships the rules                                    |
| [komiser](https://github.com/tailwarden/komiser)    | Cost dashboard with a server and a UI       | This is one binary and one command                      |
| [infracost](https://www.infracost.io)               | Cost of Terraform changes before apply      | This looks at what is already running                   |
| Trusted Advisor                                     | AWS's own                                   | Paywalled tiers, console-first, no CLI, no exit code    |

The complement worth stating: **zombie-scanner finds it, you decide, awsweeper
or aws-nuke executes.**

## Roadmap

- **v0.2 — Ignore rules.** A `zombie-scanner:keep` tag and a
  `.zombie-scanner.yaml` config, so a deliberate DR standby stops showing up.
- **v0.3 — Terraform drift.** `--tf-state` flags every live resource that
  appears in no state file. `terraform plan` catches drift in one direction;
  almost nothing catches this one.
- **v0.4 — CI mode.** A published GitHub Action, a Slack renderer, scheduled
  scans, and multi-account support via assume-role.

Deferred checks: idle RDS, empty target groups, unused security groups,
orphaned ENIs, old ECR images, unattached EFS.

## Development

```bash
make build    # compile
make test     # go test -race -count=1 ./...
make lint     # golangci-lint
make check    # everything CI runs
make cover    # coverage report
```

The codebase is one package per pipeline stage — `collect` talks to AWS,
`detect` decides what a zombie is, `price` handles money, `render` handles
output — and CI enforces that the pure stages never gain a dependency on the AWS
SDK.

## Status

Pre-1.0. The Go API and the JSON output may change between `v0.x` releases; the
JSON keys will not change within a major version.

Bug reports and detector ideas are welcome. If you find a false positive, that
is the most valuable issue you can file.

## License

MIT. See [LICENSE](LICENSE).
