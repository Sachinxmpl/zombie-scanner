package test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/awsapi/fake"
	"github.com/Sachinxmpl/zombie-scanner/collect"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/scan"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// four addresses: unassociated, on an instance, on a NAT gateway's interface, on a detached interface
func eipAddresses() []ec2types.Address {
	return []ec2types.Address{
		{AllocationId: aws.String("eipalloc-free"), PublicIp: aws.String("1.1.1.1")},
		{AllocationId: aws.String("eipalloc-inst"), PublicIp: aws.String("2.2.2.2"), AssociationId: aws.String("a-1"),
			InstanceId: aws.String("i-1"), NetworkInterfaceId: aws.String("eni-inst")},
		{AllocationId: aws.String("eipalloc-nat"), PublicIp: aws.String("3.3.3.3"), AssociationId: aws.String("a-2"),
			NetworkInterfaceId: aws.String("eni-nat")},
		{AllocationId: aws.String("eipalloc-spare"), PublicIp: aws.String("4.4.4.4"), AssociationId: aws.String("a-3"),
			NetworkInterfaceId: aws.String("eni-spare")},
	}
}

// answers only the requested interfaces whose status is available
func interfacesAPI(detached map[string]bool, asked *[][]string) func(context.Context, *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return func(_ context.Context, in *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error) {
		var ids []string
		available := false
		for _, f := range in.Filters {
			switch aws.ToString(f.Name) {
			case "network-interface-id":
				ids = f.Values
			case "status":
				available = slices.Equal(f.Values, []string{"available"})
			}
		}
		if asked != nil {
			*asked = append(*asked, ids)
		}
		out := &ec2.DescribeNetworkInterfacesOutput{}
		for _, id := range ids {
			if detached[id] || !available {
				out.NetworkInterfaces = append(out.NetworkInterfaces, ec2types.NetworkInterface{NetworkInterfaceId: aws.String(id)})
			}
		}
		return out, nil
	}
}

func eipScan(t *testing.T, api *fake.EC2) zombie.Report {
	t.Helper()
	eng := &scan.Engine{
		Accounts: []awsapi.Factory{&fake.Factory{
			Clients: awsapi.Clients{EC2: api, CW: &fake.CloudWatch{}, ELB: &fake.ELB{},
				RDS: &fake.RDS{}, EFS: &fake.EFS{}, KMS: &fake.KMS{}, ECR: &fake.ECR{}},
			Base: "us-east-1",
		}},
		Cfg:   detect.Defaults(),
		Clock: func() time.Time { return now },
	}
	rep, err := eng.Run(context.Background(), scan.Options{Only: []string{"eip-unassociated"}})
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func TestEIPOnDetachedInterface(t *testing.T) {
	var asked [][]string
	rep := eipScan(t, &fake.EC2{
		DescribeAddressesFunc: func(context.Context, *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error) {
			return &ec2.DescribeAddressesOutput{Addresses: eipAddresses()}, nil
		},
		DescribeNetworkInterfacesFunc: interfacesAPI(map[string]bool{"eni-spare": true}, &asked),
	})

	if !slices.Equal(ids(rep.Findings), []string{"eipalloc-free", "eipalloc-spare"}) {
		t.Fatalf("findings = %v, want [eipalloc-free eipalloc-spare]", ids(rep.Findings))
	}
	for _, f := range rep.Findings {
		if f.ResourceID == "eipalloc-free" && f.Confidence != zombie.High {
			t.Errorf("unassociated address: confidence %v, want HIGH", f.Confidence)
		}
		if f.ResourceID == "eipalloc-spare" {
			// a failover interface looks the same, so this one is less certain
			if f.Confidence != zombie.Medium || f.Metadata["network_interface"] != "eni-spare" {
				t.Errorf("detached interface: confidence %v, interface %q; want MEDIUM, eni-spare",
					f.Confidence, f.Metadata["network_interface"])
			}
			if f.MonthlyCost <= 0 {
				t.Errorf("detached interface: cost %v, want the Elastic IP price", f.MonthlyCost)
			}
		}
	}
	// the instance's interface is never asked about; the NAT one is, and is in use
	if len(asked) != 1 || !slices.Equal(asked[0], []string{"eni-nat", "eni-spare"}) {
		t.Errorf("asked about %v, want one call for [eni-nat eni-spare]", asked)
	}
}

// can't tell whether the interface is attached: unknown, so only the plain unassociated address is shown
func TestEIPInterfaceLookupFails(t *testing.T) {
	rep := eipScan(t, &fake.EC2{
		DescribeAddressesFunc: func(context.Context, *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error) {
			return &ec2.DescribeAddressesOutput{Addresses: eipAddresses()}, nil
		},
		DescribeNetworkInterfacesFunc: func(context.Context, *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return nil, apiErr{code: "UnauthorizedOperation"}
		},
	})

	if !slices.Equal(ids(rep.Findings), []string{"eipalloc-free"}) {
		t.Errorf("findings = %v, want [eipalloc-free]", ids(rep.Findings))
	}
	if len(rep.Errors) != 1 || rep.Errors[0].Kind != zombie.KindAccessDenied ||
		!slices.Equal(rep.Errors[0].Affects, []string{"eip-unassociated"}) {
		t.Errorf("errors = %+v, want one missing permission affecting eip-unassociated", rep.Errors)
	}
}

// most accounts have no address on a bare interface; they should not need the permission at all
func TestEIPNoInterfaceCallWhenNothingToAsk(t *testing.T) {
	api := &fake.EC2{
		DescribeAddressesFunc: func(context.Context, *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error) {
			return &ec2.DescribeAddressesOutput{Addresses: eipAddresses()[:2]}, nil
		},
		DescribeNetworkInterfacesFunc: func(context.Context, *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return nil, apiErr{code: "UnauthorizedOperation"}
		},
	}
	rep := eipScan(t, api)

	if slices.Contains(api.Calls(), "DescribeNetworkInterfaces") || len(rep.Errors) != 0 {
		t.Errorf("calls = %v, errors = %+v; want no interface lookup", api.Calls(), rep.Errors)
	}
}

func TestEIPInterfacesAskedInBatchesOf200(t *testing.T) {
	addrs := make([]zombie.Address, 0, 201)
	for i := range 201 {
		addrs = append(addrs, zombie.Address{AssociationID: "a", NetworkInterfaceID: fmt.Sprintf("eni-%d", i)})
	}
	var asked [][]string
	api := &fake.EC2{DescribeNetworkInterfacesFunc: interfacesAPI(map[string]bool{"eni-200": true}, &asked)}

	if err := collect.DetachedInterfaces(context.Background(), api, addrs); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 2 || len(asked[0]) != 200 || len(asked[1]) != 1 {
		t.Errorf("batch sizes = %d calls, want 200 then 1", len(asked))
	}
	if !addrs[200].InterfaceDetached || addrs[0].InterfaceDetached {
		t.Errorf("InterfaceDetached: first %v, last %v; want false, true", addrs[0].InterfaceDetached, addrs[200].InterfaceDetached)
	}
}
