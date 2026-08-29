package collect

import (
	"context"
	"fmt"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
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
