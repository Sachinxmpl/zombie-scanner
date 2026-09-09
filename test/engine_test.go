package test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/awsapi/fake"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/filter"
	"github.com/Sachinxmpl/zombie-scanner/scan"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// stands in for a real AWS error so classify() can reach it through errors.As
type apiErr struct{ code string }

func (e apiErr) Error() string                 { return e.code }
func (e apiErr) ErrorCode() string             { return e.code }
func (e apiErr) ErrorMessage() string          { return e.code }
func (e apiErr) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func engineWith(t *testing.T, filters ...filter.Filter) *scan.Engine {
	t.Helper()

	ec2api := &fake.EC2{
		DescribeVolumesFunc: func(context.Context, *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
			return &ec2.DescribeVolumesOutput{Volumes: []ec2types.Volume{{
				VolumeId:   aws.String("vol-1"),
				State:      ec2types.VolumeStateAvailable,
				VolumeType: ec2types.VolumeTypeGp3,
				Size:       aws.Int32(100),
				CreateTime: aws.Time(daysAgo(45)),
			}}}, nil
		},
		DescribeAddressesFunc: func(context.Context, *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error) {
			return nil, apiErr{code: "UnauthorizedOperation"}
		},
	}

	return &scan.Engine{
		AWS: &fake.Factory{
			Clients: awsapi.Clients{EC2: ec2api, CW: &fake.CloudWatch{}, ELB: &fake.ELB{}, RDS: &fake.RDS{}},
			Account: "123456789012",
			Base:    "us-east-1",
		},
		Cfg:     detect.Defaults(),
		Filters: filters,
		Clock:   func() time.Time { return now },
	}
}

// One denied call must degrade the report, never abort it - and the summary
// must describe exactly what the table shows.
func TestDeniedCallDegradesAndSummaryMatches(t *testing.T) {
	report, err := engineWith(t).Run(context.Background(), scan.Options{})
	if err != nil {
		t.Fatalf("a denied call must not abort the run: %v", err)
	}

	if len(report.Findings) != 1 {
		t.Errorf("got %d findings, want 1 - the EBS check was lost", len(report.Findings))
	}
	if len(report.Errors) != 1 {
		t.Fatalf("got %d errors, want 1", len(report.Errors))
	}

	e := report.Errors[0]
	if e.Service+":"+e.Operation != "ec2:DescribeAddresses" {
		t.Errorf("error not attributed: %+v", e)
	}
	// only passes if collect wrapped with %w all the way up
	if e.Kind != zombie.KindAccessDenied {
		t.Errorf("Kind = %q, want access_denied", e.Kind)
	}

	if report.Summary.ZombieCount != len(report.Findings) {
		t.Errorf("summary says %d, table shows %d",
			report.Summary.ZombieCount, len(report.Findings))
	}
}

// A filter that hides everything must not read as a clean account.
func TestFilteringIsReflectedInTheSummary(t *testing.T) {
	report, err := engineWith(t, filter.MinCost{USD: 1000}).Run(context.Background(), scan.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Findings) != 0 {
		t.Errorf("got %d findings, want 0", len(report.Findings))
	}
	if report.Summary.ZombieCount != 0 || report.Summary.TotalMonthlyUSD != 0 {
		t.Errorf("summary = %+v, want zeroed", report.Summary)
	}
	if report.Filtered["--min-cost"] != 1 {
		t.Errorf("Filtered = %v, want 1 hidden by --min-cost", report.Filtered)
	}
}

// The JSON contract: empty collections must marshal to [] and {}, never null.
func TestEmptyReportIsStillValid(t *testing.T) {
	eng := &scan.Engine{
		AWS: &fake.Factory{Clients: awsapi.Clients{
			EC2: &fake.EC2{}, CW: &fake.CloudWatch{}, ELB: &fake.ELB{}, RDS: &fake.RDS{},
		}},
		Cfg:   detect.Defaults(),
		Clock: func() time.Time { return now },
	}

	report, err := eng.Run(context.Background(), scan.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Findings == nil || report.Errors == nil {
		t.Error("nil slices would marshal to null and break a consumer's jq")
	}
	if report.SchemaVersion != zombie.SchemaVersion {
		t.Errorf("SchemaVersion = %q", report.SchemaVersion)
	}
}
