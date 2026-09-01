package scan

import (
	"context"
	"errors"
	"time"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/collect"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/price"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
	"github.com/aws/smithy-go"
)

// engine runs a scan
type Engine struct {
	AWS awsapi.Factory
	Cfg detect.Config

	Clock func() time.Time
}

type Options struct {
	Regions    []string
	AllRegions bool // scan every region the account has opted into
	Only, Skip []string
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
	now := e.now()

	account, err := e.AWS.AccountID(ctx)
	if err != nil {
		return zombie.Report{}, err
	}

	regions, err := e.resolveRegions(ctx, o)
	if err != nil {
		return zombie.Report{}, err
	}

	report := zombie.Report{
		AccountID: account,
		ScannedAt: now,
		Regions:   regions,
		Findings:  []zombie.Finding{},
		Errors:    []zombie.ScanError{},
	}

	for _, region := range regions {
		found, errs := e.scanOneRegion(ctx, region, account, now, o)
		report.Findings = append(report.Findings, found...)
		report.Errors = append(report.Errors, errs...)
	}

	report.Summary = summarize(report.Findings)
	report.Normalize() // the never-null guarantee, once, at the end
	return report, nil
}

func (e *Engine) resolveRegions(ctx context.Context, o Options) ([]string, error) {
	switch {
	case o.AllRegions:
		rs, err := e.AWS.Regions(ctx)
		if err != nil {
			return nil, err
		}
		return rs, nil
	case len(o.Regions) > 0:
		return o.Regions, nil
	default:
		return []string{e.AWS.BaseRegion()}, nil
	}
}

// step is one API call
type step struct {
	service   string
	operation string
	run       func(ctx context.Context, inv *zombie.Inventory) error
}

// scans one region, returns errors as data
func (e *Engine) scanOneRegion(ctx context.Context, region, account string, now time.Time, o Options) ([]zombie.Finding, []zombie.ScanError) {
	errs := []zombie.ScanError{}

	clients, err := e.AWS.For(ctx, region)
	if err != nil {
		return nil, append(errs, newScanError(region, "aws", "Clients", err))
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
	}

	inv.Failed = map[string]bool{}

	for _, s := range steps {
		if err := s.run(ctx, &inv); err != nil {
			inv.Failed[s.service+":"+s.operation] = true
			errs = append(errs, newScanError(region, s.service, s.operation, err))
			continue // degrade, never abort
		}
	}

	findings := price.Apply(detect.Run(inv, e.Cfg, o.Only, o.Skip), region)
	return findings, errs
}

func newScanError(region, service, operation string, err error) zombie.ScanError {
	return zombie.ScanError{
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
