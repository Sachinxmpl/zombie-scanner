package collect

import (
	"context"
	"fmt"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Returns every EBS volume in the region
func Volumes(ctx context.Context, api awsapi.EC2API) ([]zombie.Volume, error) {
	out := []zombie.Volume{}

	p := ec2.NewDescribeVolumesPaginator(api, &ec2.DescribeVolumesInput{})

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ec2:DescribeVolumes: %w", err)
		}
		for _, v := range page.Volumes {
			out = append(out, toVolume(v))
		}
	}
	return out, nil
}

// Returns every Elastic IP address in the region
func Addresses(ctx context.Context, api awsapi.EC2API) ([]zombie.Address, error) {
	out := []zombie.Address{}

	resp, err := api.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, fmt.Errorf("ec2:DescribeAddresses: %w", err)
	}
	for _, a := range resp.Addresses {
		out = append(out, toAddress(a))
	}

	return out, nil
}

// Returns every EBS snapshot this account owns in the region
func Snapshots(ctx context.Context, api awsapi.EC2API) ([]zombie.Snapshot, error) {
	out := []zombie.Snapshot{}

	p := ec2.NewDescribeSnapshotsPaginator(api, &ec2.DescribeSnapshotsInput{
		OwnerIds: []string{"self"},
	})

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ec2:DescribeSnapshots: %w", err)
		}
		for _, s := range page.Snapshots {
			out = append(out, toSnapshot(s))
		}
	}
	return out, nil
}

// Returns every AMI this account owns
func Images(ctx context.Context, api awsapi.EC2API) ([]zombie.Image, error) {
	out := []zombie.Image{}

	p := ec2.NewDescribeImagesPaginator(api, &ec2.DescribeImagesInput{
		Owners: []string{"self"},
	})

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ec2:DescribeImages: %w", err)
		}
		for _, i := range page.Images {
			out = append(out, toImage(i))
		}
	}
	return out, nil
}

// Returns stopped instances only. Filtered server-side
func StoppedInstances(ctx context.Context, api awsapi.EC2API) ([]zombie.Instance, error) {
	out := []zombie.Instance{}

	p := ec2.NewDescribeInstancesPaginator(api, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"stopped"},
			},
		},
	})

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ec2:DescribeInstances: %w", err)
		}
		for _, r := range page.Reservations {
			for _, i := range r.Instances {
				out = append(out, toInstance(i))
			}
		}
	}
	return out, nil
}
