package scan

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/collect"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/filter"
	"github.com/Sachinxmpl/zombie-scanner/price"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
	"github.com/aws/smithy-go"
	"golang.org/x/sync/errgroup"
)

// engine runs a scan
type Engine struct {
	Accounts []awsapi.Factory
	Cfg      detect.Config
	Filters  []filter.Filter
	Log      *slog.Logger

	// no of parallel regions scans.
	Concurrency int

	Clock func() time.Time
}

type Options struct {
	Regions    []string
	AllRegions bool // scan every region the account has opted into
	Only, Skip []string
}

const defaultConcurrency = 8

func (e *Engine) concurrency() int {
	if e.Concurrency > 0 {
		return e.Concurrency
	}
	return defaultConcurrency
}

func (e *Engine) log() *slog.Logger {
	if e.Log != nil {
		return e.Log
	}
	return slog.New(slog.DiscardHandler)
}

func (e *Engine) now() time.Time {
	if e.Clock != nil {
		return e.Clock()
	}
	return time.Now().UTC()
}

// Performs a complete scan
// Returns error only for failures that make the whole run meaningless (no crendential, or no discoverable regionss). Everything else lands in a Report.Errrors and scan continues
func (e *Engine) Run(ctx context.Context, o Options) (zombie.Report, error) {
	if len(e.Accounts) == 0 {
		return zombie.Report{}, errors.New("scan: no accounts configured")
	}

	now := e.now()

	type target struct {
		aws     awsapi.Factory
		account string
		region  string
	}

	var (
		targets  []target
		accounts []string
		regions  []string
		seenReg  = map[string]bool{}
		preErrs  []zombie.ScanError
	)

	for _, f := range e.Accounts {
		account, err := f.AccountID(ctx)
		if err != nil {
			preErrs = append(
				preErrs,
				newScanError("", "", "sts", "GetCallerIdentity", err),
			)
			continue
		}
		rs, err := e.resolveRegions(ctx, f, o)
		if err != nil {
			preErrs = append(preErrs, newScanError(account, "", "ec2", "DescribeRegions", err))
			continue
		}
		accounts = append(accounts, account)
		for _, r := range rs {
			targets = append(targets, target{aws: f, account: account, region: r})
			if !seenReg[r] {
				seenReg[r] = true
				regions = append(regions, r)
			}
		}
	}

	if len(accounts) == 0 {
		return zombie.Report{}, fmt.Errorf("no account could be scanned: %w", preErrs[0])
	}

	report := zombie.Report{
		AccountID: accounts[0],
		Accounts:  accounts,
		ScannedAt: now,
		Regions:   regions,
		Findings:  []zombie.Finding{},
		Errors:    []zombie.ScanError{},
	}

	var (
		mu       sync.Mutex
		findings []zombie.Finding
		scanErrs = preErrs
		filtered = map[string]int{}
	)

	g, gctx := errgroup.WithContext(ctx)
	// one budget for accounts x regions, not per account
	g.SetLimit(e.concurrency())

	for _, t := range targets {
		g.Go(func() error {
			start := time.Now()
			found, dropped, errs := e.scanOneRegion(gctx, t.aws, t.region, t.account, now, o)

			mu.Lock()
			findings = append(findings, found...)
			scanErrs = append(scanErrs, errs...)
			for name, n := range dropped {
				filtered[name] += n
			}
			mu.Unlock()

			e.log().Debug("region scanned", "account", t.account, "region", t.region, "findings", len(found), "errors", len(errs), "took", time.Since(start))

			// nil -> returning error would cancel gctx
			return nil
		})
	}
	_ = g.Wait()

	report.Findings = findings
	report.Errors = scanErrs
	if len(filtered) > 0 {
		report.Filtered = filtered
	}
	report.Summary = summarize(report.Findings)
	report.Normalize() // the never-null guarantee
	return report, nil
}

func (e *Engine) resolveRegions(ctx context.Context, aws awsapi.Factory, o Options) ([]string, error) {
	switch {
	case o.AllRegions:
		rs, err := aws.Regions(ctx)
		if err != nil {
			return nil, err
		}
		return rs, nil
	case len(o.Regions) > 0:
		return o.Regions, nil
	default:
		return []string{aws.BaseRegion()}, nil
	}
}

// step is one API call
type step struct {
	service   string
	operation string
	run       func(ctx context.Context, inv *zombie.Inventory) error
}

// scans one region, returns errors as data
func (e *Engine) scanOneRegion(ctx context.Context, aws awsapi.Factory, region, account string, now time.Time, o Options) ([]zombie.Finding, map[string]int, []zombie.ScanError) {
	errs := []zombie.ScanError{}

	clients, err := aws.For(ctx, region)
	if err != nil {
		return nil, nil, append(errs, newScanError(account, region, "aws", "Clients", err))
	}

	inv := zombie.Inventory{
		Region:    region,
		AccountID: account,
		Now:       now,
	}

	steps := []step{
		{"ec2", "DescribeVolumes", func(ctx context.Context, inv *zombie.Inventory) error {
			v, err := collect.Volumes(ctx, clients.EC2)
			inv.Volumes = v // nil on error - detectors range over it zero times
			return err
		}},
		{"rds", "DescribeDBInstances", func(ctx context.Context, inv *zombie.Inventory) error {
			dbs, err := collect.DBInstances(ctx, clients.RDS)
			inv.DBInstances = dbs
			return err
		}},
		{"ec2", "DescribeAddresses", func(ctx context.Context, inv *zombie.Inventory) error {
			a, err := collect.Addresses(ctx, clients.EC2)
			inv.Addresses = a
			return err
		}},
		{"ec2", "DescribeSnapshots", func(ctx context.Context, inv *zombie.Inventory) error {
			s, err := collect.Snapshots(ctx, clients.EC2)
			inv.Snapshots = s
			return err
		}},
		{"ec2", "DescribeInstances", func(ctx context.Context, inv *zombie.Inventory) error {
			i, err := collect.StoppedInstances(ctx, clients.EC2)
			inv.Instances = i
			return err
		}},
		{"ec2", "DescribeImages", func(ctx context.Context, inv *zombie.Inventory) error {
			i, err := collect.Images(ctx, clients.EC2)
			inv.Images = i
			return err
		}},
		{"ec2", "DescribeNatGateways", func(ctx context.Context, inv *zombie.Inventory) error {
			n, err := collect.NatGateways(ctx, clients.EC2)
			inv.NATGateways = n
			return err
		}},
		{"rds", "DescribeDBSnapshots", func(ctx context.Context, inv *zombie.Inventory) error {
			s, err := collect.DBSnapshots(ctx, clients.RDS)
			inv.DBSnapshots = s
			return err
		}},
		{"elasticfilesystem", "DescribeFileSystems", func(ctx context.Context, inv *zombie.Inventory) error {
			fs, err := collect.FileSystems(ctx, clients.EFS)
			inv.FileSystems = fs
			return err
		}},
		{"elasticloadbalancing", "DescribeLoadBalancers", func(ctx context.Context, inv *zombie.Inventory) error {
			lbs, err := collect.LoadBalancers(ctx, clients.ELB)
			inv.LoadBalancers = lbs
			return err
		}},
		{"elasticloadbalancing", "DescribeTags", func(ctx context.Context, inv *zombie.Inventory) error {
			return collect.LoadBalancerTags(ctx, clients.ELB, inv.LoadBalancers)
		}},
		// must run after every collector that it builds queries from
		{"cloudwatch", "GetMetricData", func(ctx context.Context, inv *zombie.Inventory) error {
			queries := make([]collect.Query, 0,
				len(inv.NATGateways)+len(inv.LoadBalancers)+len(inv.DBInstances))

			for _, n := range inv.NATGateways {
				queries = append(queries, collect.Query{
					Namespace:  "AWS/NATGateway",
					Metric:     "BytesOutToDestination",
					Dimension:  "NatGatewayId",
					ResourceID: n.ID,
				})
			}
			for _, lb := range inv.LoadBalancers {
				if lb.Type != "application" || lb.MetricSuffix == "" {
					continue
				}
				queries = append(queries, collect.Query{
					Namespace:  "AWS/ApplicationELB",
					Metric:     "RequestCount",
					Dimension:  "LoadBalancer",
					ResourceID: lb.MetricSuffix, // never lb.ARN
				})
			}

			// stopped instances publish nothing, so querying them wastes a slot in the batch
			for _, db := range inv.DBInstances {
				if db.Status != "available" {
					continue
				}
				queries = append(queries, collect.Query{
					Namespace:  "AWS/RDS",
					Metric:     "DatabaseConnections",
					Dimension:  "DBInstanceIdentifier",
					ResourceID: db.ID,
				})
			}

			window := time.Duration(e.Cfg.IdleWindowDays) * 24 * time.Hour
			ms, err := collect.MetricSums(ctx, clients.CW, inv.Now, window, queries)
			inv.Metrics = ms
			return err
		}},
	}

	inv.Failed = map[string]bool{}

	for _, s := range steps {
		t0 := time.Now()
		if err := s.run(ctx, &inv); err != nil {
			e.log().Debug("step failed", "op", s.service+":"+s.operation, "region", region, "err", err)
			inv.Failed[s.service+":"+s.operation] = true
			errs = append(errs, newScanError(account, region, s.service, s.operation, err))
			continue // degrade, never abort
		}
		e.log().Debug("step ok", "op", s.service+":"+s.operation, "took", time.Since(t0))
	}

	findings := price.Apply(detect.Run(inv, e.Cfg, o.Only, o.Skip), region)
	findings, dropped := filter.Apply(findings, e.Filters)
	return findings, dropped, errs
}

func newScanError(account, region, service, operation string, err error) zombie.ScanError {
	return zombie.ScanError{
		Account:   account,
		Region:    region,
		Service:   service,
		Operation: operation,
		Kind:      classify(err),
		Message:   err.Error(),
	}
}

func classify(err error) zombie.ErrorKind {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return zombie.KindOther
	}

	switch apiErr.ErrorCode() {
	case "AccessDenied", "AccessDeniedException", "UnauthorizedOperation", "AuthFailure":
		return zombie.KindAccessDenied
	case "Throttling", "ThrottlingException", "RequestLimitExceeded", "TooManyRequestsException":
		return zombie.KindThrottled
	case "InvalidAction", "UnsupportedOperation", "OptInRequired", "InvalidClientTokenId":
		return zombie.KindUnsupported
	default:
		return zombie.KindOther
	}
}

// summarize folds the findings into the report header numbers.
func summarize(fs []zombie.Finding) zombie.Summary {
	s := zombie.Summary{
		ZombieCount:  len(fs),
		ByConfidence: map[string]int{},
		ByDetector:   map[string]float64{},
	}
	for _, f := range fs {
		s.TotalMonthlyUSD += f.MonthlyCost
		s.ByConfidence[f.Confidence.String()]++
		s.ByDetector[f.Detector] += f.MonthlyCost
	}
	return s
}
