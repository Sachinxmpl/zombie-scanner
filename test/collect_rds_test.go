package test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/awsapi/fake"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/scan"
)

func TestRDSMetricQueryUsesTheRightDimension(t *testing.T) {
	var exprs []string

	rdsapi := &fake.RDS{
		DescribeDBInstancesFunc: func(context.Context, *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
			return &rds.DescribeDBInstancesOutput{DBInstances: []rdstypes.DBInstance{
				{
					DBInstanceIdentifier: aws.String("db-running"),
					DBInstanceStatus:     aws.String("available"),
					Engine:               aws.String("postgres"),
					DBInstanceClass:      aws.String("db.t3.micro"),
					StorageType:          aws.String("gp3"),
					AllocatedStorage:     aws.Int32(20),
					InstanceCreateTime:   aws.Time(daysAgo(200)),
				},
				{
					DBInstanceIdentifier: aws.String("db-stopped"),
					DBInstanceStatus:     aws.String("stopped"),
					Engine:               aws.String("postgres"),
					DBInstanceClass:      aws.String("db.t3.micro"),
					StorageType:          aws.String("gp3"),
					AllocatedStorage:     aws.Int32(20),
					InstanceCreateTime:   aws.Time(daysAgo(200)),
				},
			}}, nil
		},
	}

	cw := &fake.CloudWatch{
		GetMetricDataFunc: func(_ context.Context, in *cloudwatch.GetMetricDataInput) (*cloudwatch.GetMetricDataOutput, error) {
			for _, q := range in.MetricDataQueries {
				exprs = append(exprs, aws.ToString(q.Expression))
			}
			return &cloudwatch.GetMetricDataOutput{}, nil
		},
	}

	eng := &scan.Engine{
		Accounts: []awsapi.Factory{
			&fake.Factory{
				Clients: awsapi.Clients{
					EC2: &fake.EC2{}, CW: cw, ELB: &fake.ELB{}, RDS: rdsapi, EFS: &fake.EFS{}, KMS: &fake.KMS{},
				},
				Base: "us-east-1",
			},
		},
		Cfg:   detect.Defaults(),
		Clock: func() time.Time { return now },
	}

	if _, err := eng.Run(context.Background(), scan.Options{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// the stopped instance publishes nothing, so it must not be queried
	if len(exprs) != 1 {
		t.Fatalf("got %d metric queries, want 1: %v", len(exprs), exprs)
	}
	expr := exprs[0]

	for _, want := range []string{
		`Namespace="AWS/RDS"`,
		`MetricName="DatabaseConnections"`,
		`DBInstanceIdentifier="db-running"`,
	} {
		if !strings.Contains(expr, want) {
			t.Errorf("expression missing %s\ngot: %s", want, expr)
		}
	}

	// SEARCH matches on the id whatever other dimensions AWS attaches. A
	// MetricStat query must name the dimension set exactly, and returns
	// nothing - indistinguishable from idle - when it guesses wrong.
	if !strings.HasPrefix(expr, "SUM(SEARCH(") {
		t.Errorf("not a SEARCH expression: %s", expr)
	}
}
