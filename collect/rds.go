package collect

import (
	"context"
	"fmt"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

// Returns every RDS DB instance in the region
func DBInstances(ctx context.Context, api awsapi.RDSAPI) ([]zombie.DBInstance, error) {
	out := []zombie.DBInstance{}
	p := rds.NewDescribeDBInstancesPaginator(api, &rds.DescribeDBInstancesInput{})

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("rds:DescribeDBInstances: %w", err)
		}
		for _, db := range page.DBInstances {
			out = append(out, toDBInstance(db))
		}
	}
	return out, nil
}

// Returns manaul DB snapshots.
// Automated Snapshots are not included because they are automatically deleted by AWS after a retention period, and therefore cannot be considered "zombies".
func DBSnapshots(ctx context.Context, api awsapi.RDSAPI) ([]zombie.DBSnapshot, error) {
	out := []zombie.DBSnapshot{}

	p := rds.NewDescribeDBSnapshotsPaginator(api, &rds.DescribeDBSnapshotsInput{
		SnapshotType: aws.String("manual"),
	})

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("rds:DescribeDBSnapshots: %w", err)
		}
		for _, s := range page.DBSnapshots {
			out = append(out, toDBSnapshot(s))
		}
	}
	return out, nil
}
