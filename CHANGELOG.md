# Changelog

All notable changes to this project are listed here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Before 1.0, the JSON
output's `schema_version` is what tells you whether scripts will keep working.

## [Unreleased]

### Added

- `--rates <file>` (also `ZOMBIE_SCANNER_RATES` and `rates:` in the config
  file) replaces built-in prices with your own, e.g. after a discount. List
  only the prices that differ. A misspelt key or a negative price is an error.
  The report says which prices were used: a new `prices` field in the JSON
  (nothing removed, so `schema_version` is still `"1"`) and a note in the
  footer.

### Security

- The tool now refuses any AWS call that isn't a `Describe`, `Get` or `List`
  while it runs, before anything is sent. Until now this was only checked in
  CI. `sts:AssumeRole`, used by `--accounts`, is the one exception.

### Fixed

- Dollar amounts in the JSON report are rounded to cents (`1.64` instead of
  `1.6400000000000001`).
- `--all-regions` failed with "if any flags in the group [region all-regions]
  are set…" when the config file or `ZOMBIE_SCANNER_REGION` set a region. That
  region is now only the starting region. Typing `--region` and `--all-regions`
  together is still an error.
- `instance-stopped` now adds the cost of Elastic IPs attached to a stopped
  instance ($3.65/month each). Before, it counted only the disks. A stopped
  instance with no disks but an Elastic IP is now reported too.

## [0.2.0] - 2026-10-23

**Twelve checks, ways to hide what you want to keep, several accounts at once,
and a GitHub Action.**

### Added

**Six new checks** (six became twelve):

- **`rds-stopped`** (HIGH): stopped databases. Storage keeps billing while they
  are stopped, and AWS restarts them by itself after 7 days. The finding says
  when.
- **`rds-idle`** (MEDIUM): running databases nothing has connected to. If the
  tool has no price for the database type, or it's Oracle or SQL Server, it says
  so instead of guessing.
- **`efs-unused`** (HIGH): file systems no server can reach. Priced from the
  exact size AWS reports for each storage tier.
- **`kms-key-disabled`** (HIGH): disabled encryption keys you created, $1/month
  each. Keys AWS manages, and keys already scheduled for deletion, are skipped.
- **`rds-snapshot-aged`** (LOW): old manual database snapshots. Automatic ones
  expire by themselves, so they are skipped.
- **`ecr-untagged`** (LOW): old untagged container images. MEDIUM if the image
  was last pulled long ago. No pull on record doesn't count as "never pulled",
  because ECR hasn't always recorded pulls.

If one KMS key or ECR repository can't be read (for example, its own policy
blocks the tool), the others are still checked and the report says how many
were skipped.

**Three ways to hide what you want to keep:**

- **The `zombie-scanner:keep` tag.** Tag a resource and it stops showing. Any
  value works; use it to write why. Change the tag with `--keep-tag`, or show
  everything with `--no-keep-tag`.
- **A `.zombie-scanner.yaml` file** with your settings and ignore rules. Each rule
  matches on `id`, `detector` or `region` and needs a `reason`. Rules that match
  nothing are reported, and a misspelt setting is an error.
- **`--tf-state`** hides everything your Terraform state manages, leaving what
  nobody owns.

Hidden findings are always counted in the report, e.g. "3 hidden by --min-cost".

**More accounts, more output formats:**

- **`--accounts` and `--role-name`** scan several accounts in one run by assuming a
  read-only role in each. The report gets an Account column. An account that
  can't be reached is reported as a skipped check; the rest still run.
  `zombie-scanner iam-policy --assume-role` prints the permission your own login
  needs to assume those roles. The JSON report gains an `accounts` list, and each
  error an `account`. Nothing was renamed or removed, so `schema_version` is
  still `"1"`.
- **`--output markdown`** for GitHub job summaries and PR comments. Long reports
  are cut to 100 rows and say so; the totals stay correct.
- **`zombie-scanner render`** prints a saved JSON report as a table, JSON or
  Markdown, without calling AWS.
- **A GitHub Action** (`uses: Sachinxmpl/zombie-scanner@v0.2.0`). It checks the
  download's checksum and build attestation before running it. A full weekly
  example is in `docs/examples/scheduled-scan.yml`.

### Fixed

- `--idle-window-days 0` (or a negative `--stopped-days` or `--snapshot-age-days`)
  started the scan and then failed partway through. Bad values are now rejected
  before anything is sent to AWS.

### Security

- CI now also checks that only the `awsapi` package can assume roles, and that
  no EFS, KMS or ECR write call appears in the code.

### Permissions

Eight new read-only permissions. If you wrote your own IAM policy, add these, or
regenerate it with `zombie-scanner iam-policy`:

```
ecr:DescribeImages                       kms:DescribeKey
ecr:DescribeRepositories                 kms:ListKeys
elasticfilesystem:DescribeFileSystems    rds:DescribeDBInstances
elasticloadbalancing:DescribeTags        rds:DescribeDBSnapshots
```

Without them the new checks are skipped with a note, and the old ones still run.

## [0.1.1] - 2026-09-04

### Fixed

- `zombie-scanner version` printed `dev (none)` after `go install`. It now reads
  the version Go stores in the binary.
- The Linux package install commands now use the right download links.

## [0.1.0] - 2026-09-04

First release. Six checks (`ebs-unattached`, `eip-unassociated`,
`instance-stopped`, `nat-idle`, `elb-idle`, `snapshot-aged`), table and JSON
output, several regions at once, confidence levels, a generated IAM policy, and
signed releases. Read-only, and checked in CI.

[Unreleased]: https://github.com/Sachinxmpl/zombie-scanner/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/Sachinxmpl/zombie-scanner/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/Sachinxmpl/zombie-scanner/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/Sachinxmpl/zombie-scanner/releases/tag/v0.1.0
