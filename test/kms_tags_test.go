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
	"github.com/Sachinxmpl/zombie-scanner/filter"
	"github.com/Sachinxmpl/zombie-scanner/scan"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func kmsWithTags(tagsFor func(id string) ([]kmstypes.Tag, error)) *fake.KMS {
	return &fake.KMS{
		ListKeysFunc: func(context.Context, *kms.ListKeysInput) (*kms.ListKeysOutput, error) {
			return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{
				{KeyId: aws.String("key-kept")},
				{KeyId: aws.String("key-plain")},
				{KeyId: aws.String("key-aws")},
			}}, nil
		},
		DescribeKeyFunc: func(_ context.Context, in *kms.DescribeKeyInput) (*kms.DescribeKeyOutput, error) {
			manager := kmstypes.KeyManagerTypeCustomer
			if aws.ToString(in.KeyId) == "key-aws" {
				manager = kmstypes.KeyManagerTypeAws
			}
			return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{
				KeyId:        in.KeyId,
				KeyManager:   manager,
				KeyState:     kmstypes.KeyStateDisabled,
				CreationDate: aws.Time(daysAgo(400)),
			}}, nil
		},
		ListResourceTagsFunc: func(_ context.Context, in *kms.ListResourceTagsInput) (*kms.ListResourceTagsOutput, error) {
			tags, err := tagsFor(aws.ToString(in.KeyId))
			if err != nil {
				return nil, err
			}
			return &kms.ListResourceTagsOutput{Tags: tags}, nil
		},
	}
}

func keepTagOnKept(id string) ([]kmstypes.Tag, error) {
	if id == "key-kept" {
		return []kmstypes.Tag{{TagKey: aws.String(filter.DefaultKeepTag), TagValue: aws.String("DR")}}, nil
	}
	return nil, nil
}

func kmsScan(t *testing.T, api *fake.KMS) zombie.Report {
	t.Helper()
	eng := &scan.Engine{
		Accounts: []awsapi.Factory{&fake.Factory{
			Clients: awsapi.Clients{EC2: &fake.EC2{}, CW: &fake.CloudWatch{}, ELB: &fake.ELB{},
				RDS: &fake.RDS{}, EFS: &fake.EFS{}, KMS: api, ECR: &fake.ECR{}},
			Base: "us-east-1",
		}},
		Cfg:     detect.Defaults(),
		Filters: []filter.Filter{filter.KeepTag{Key: filter.DefaultKeepTag}},
		Clock:   func() time.Time { return now },
	}
	rep, err := eng.Run(context.Background(), scan.Options{Only: []string{"kms-key-disabled"}})
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func TestKMSKeepTagHidesKey(t *testing.T) {
	api := kmsWithTags(keepTagOnKept)
	rep := kmsScan(t, api)

	if !slices.Equal(ids(rep.Findings), []string{"key-plain"}) {
		t.Errorf("findings = %v, want [key-plain]", ids(rep.Findings))
	}
	if len(rep.Errors) != 0 {
		t.Errorf("errors = %+v, want none", rep.Errors)
	}
	// AWS-managed keys can't be tagged, so asking costs a call for nothing
	asked := 0
	for _, c := range api.Calls() {
		if c == "ListResourceTags" {
			asked++
		}
	}
	if asked != 2 {
		t.Errorf("ListResourceTags called %d times, want 2 (customer keys only)", asked)
	}
}

func TestKMSKeyWithUnreadableTagsIsSkipped(t *testing.T) {
	rep := kmsScan(t, kmsWithTags(func(id string) ([]kmstypes.Tag, error) {
		if id == "key-kept" {
			return nil, apiErr{code: "AccessDeniedException"}
		}
		return nil, nil
	}))

	if !slices.Equal(ids(rep.Findings), []string{"key-plain"}) {
		t.Errorf("findings = %v, want [key-plain]", ids(rep.Findings))
	}
	if len(rep.Errors) != 1 || rep.Errors[0].Operation != "ListResourceTags" {
		t.Fatalf("errors = %+v, want one ListResourceTags warning", rep.Errors)
	}
	if rep.Errors[0].Kind == zombie.KindAccessDenied {
		t.Errorf("one key's policy said no; reported as a missing IAM permission: %+v", rep.Errors[0])
	}
}

func TestKMSTagsDeniedForEveryKey(t *testing.T) {
	rep := kmsScan(t, kmsWithTags(func(string) ([]kmstypes.Tag, error) {
		return nil, apiErr{code: "AccessDeniedException"}
	}))

	if len(rep.Findings) != 0 {
		t.Errorf("findings = %v, want none", ids(rep.Findings))
	}
	if len(rep.Errors) != 1 || rep.Errors[0].Kind != zombie.KindAccessDenied ||
		!slices.Equal(rep.Errors[0].Affects, []string{"kms-key-disabled"}) {
		t.Errorf("errors = %+v, want one missing permission affecting kms-key-disabled", rep.Errors)
	}
}

func TestKMSKeyTagsPartialCounts(t *testing.T) {
	keys := []zombie.KMSKey{
		{ID: "a", Manager: "CUSTOMER"}, {ID: "b", Manager: "CUSTOMER"}, {ID: "c", Manager: "AWS"},
	}
	err := collect.KMSKeyTags(context.Background(), kmsWithTags(func(id string) ([]kmstypes.Tag, error) {
		if id == "a" {
			return nil, apiErr{code: "AccessDeniedException"}
		}
		return nil, nil
	}), keys)

	var partial *collect.Partial
	if !errors.As(err, &partial) || partial.Skipped != 1 || partial.Total != 2 {
		t.Fatalf("err = %v, want a Partial for 1 of 2 keys", err)
	}
	if !keys[0].TagsUnknown || keys[1].TagsUnknown || keys[2].TagsUnknown {
		t.Errorf("TagsUnknown = %v %v %v, want true false false",
			keys[0].TagsUnknown, keys[1].TagsUnknown, keys[2].TagsUnknown)
	}
}
