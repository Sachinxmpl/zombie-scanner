package test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/awsapi/fake"
	"github.com/Sachinxmpl/zombie-scanner/collect"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/price"
	"github.com/Sachinxmpl/zombie-scanner/scan"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// AWS-managed keys are free and undeletable; PendingDeletion is already on its
// way out. Only a disabled customer-managed key is money you can stop paying.
func TestKMSKeyDisabled(t *testing.T) {
	inv := zombie.Inventory{Now: now, KMSKeys: []zombie.KMSKey{
		{ID: "key-disabled", Manager: "CUSTOMER", State: "Disabled",
			Description: "old app", CreatedAt: daysAgo(400)},
		{ID: "key-enabled", Manager: "CUSTOMER", State: "Enabled", CreatedAt: daysAgo(400)},
		{ID: "key-pending", Manager: "CUSTOMER", State: "PendingDeletion", CreatedAt: daysAgo(400)},
		{ID: "key-aws", Manager: "AWS", State: "Disabled", CreatedAt: daysAgo(400)},
	}}

	got := detect.Run(inv, detect.Defaults(), []string{"kms-key-disabled"}, nil)

	if !slices.Equal(ids(got), []string{"key-disabled"}) {
		t.Fatalf("got %v, want [key-disabled]", ids(got))
	}
	if got[0].Confidence != zombie.High {
		t.Errorf("confidence = %v, want HIGH", got[0].Confidence)
	}

	// a region with a multiplier: the flat fee must not move with it
	priced := price.Apply(got, "ap-south-1")
	if priced[0].MonthlyCost != 1.0 {
		t.Errorf("cost = %v, want a flat $1", priced[0].MonthlyCost)
	}
}

func oneDeniedKey() *fake.KMS {
	return &fake.KMS{
		ListKeysFunc: func(context.Context, *kms.ListKeysInput) (*kms.ListKeysOutput, error) {
			return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{
				{KeyId: aws.String("key-ok")},
				{KeyId: aws.String("key-denied")},
			}}, nil
		},
		DescribeKeyFunc: func(_ context.Context, in *kms.DescribeKeyInput) (*kms.DescribeKeyOutput, error) {
			if aws.ToString(in.KeyId) == "key-denied" {
				return nil, apiErr{code: "AccessDeniedException"}
			}
			return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{
				KeyId:        in.KeyId,
				KeyManager:   kmstypes.KeyManagerTypeCustomer,
				KeyState:     kmstypes.KeyStateDisabled,
				CreationDate: aws.Time(daysAgo(400)),
			}}, nil
		},
	}
}

// One key with a restrictive policy must not hide every other key, and the
// gap must still be visible: "no zombies" and "could not look" differ.
func TestKMSSkipsKeysItCannotDescribe(t *testing.T) {
	got, err := collect.KMSKeys(context.Background(), oneDeniedKey())

	if len(got) != 1 || got[0].ID != "key-ok" {
		t.Fatalf("got %+v, want just key-ok", got)
	}
	var partial *collect.Partial
	if !errors.As(err, &partial) {
		t.Fatalf("err = %v, want a Partial reporting the skipped key", err)
	}
	if partial.Skipped != 1 || partial.Total != 2 {
		t.Errorf("skipped %d of %d, want 1 of 2", partial.Skipped, partial.Total)
	}
}

func TestKMSListFailureIsNotPartial(t *testing.T) {
	api := &fake.KMS{
		ListKeysFunc: func(context.Context, *kms.ListKeysInput) (*kms.ListKeysOutput, error) {
			return nil, apiErr{code: "AccessDeniedException"}
		},
	}
	_, err := collect.KMSKeys(context.Background(), api)

	var partial *collect.Partial
	if err == nil || errors.As(err, &partial) {
		t.Fatalf("err = %v, want a plain failure: the whole check could not run", err)
	}
}

// End to end: the readable key is still reported, the skipped one is a
// warning naming DescribeKey, and it is not shown as a missing IAM permission.
func TestPartialKMSReadKeepsFindingsAndWarns(t *testing.T) {
	eng := &scan.Engine{
		Accounts: []awsapi.Factory{&fake.Factory{
			Clients: awsapi.Clients{EC2: &fake.EC2{}, CW: &fake.CloudWatch{}, ELB: &fake.ELB{},
				RDS: &fake.RDS{}, EFS: &fake.EFS{}, KMS: oneDeniedKey()},
			Base: "us-east-1",
		}},
		Cfg:   detect.Defaults(),
		Clock: func() time.Time { return now },
	}

	rep, err := eng.Run(context.Background(), scan.Options{Only: []string{"kms-key-disabled"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids(rep.Findings), []string{"key-ok"}) {
		t.Errorf("findings = %v, want [key-ok]", ids(rep.Findings))
	}
	if len(rep.Errors) != 1 || rep.Errors[0].Operation != "DescribeKey" {
		t.Fatalf("errors = %+v, want one DescribeKey warning", rep.Errors)
	}
	// IAM already allows it; a key policy said no, so "grant kms:DescribeKey" would mislead
	if rep.Errors[0].Kind == zombie.KindAccessDenied {
		t.Errorf("reported as a missing IAM permission: %+v", rep.Errors[0])
	}
}
