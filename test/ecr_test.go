package test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/Sachinxmpl/zombie-scanner/awsapi/fake"
	"github.com/Sachinxmpl/zombie-scanner/collect"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/price"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// ECR only records pulls since it started tracking them, so a missing pull
// time must never raise confidence.
func TestECRUntaggedTreatsMissingPullTimeAsUnknown(t *testing.T) {
	longAgo := daysAgo(300)
	recent := daysAgo(2)

	inv := zombie.Inventory{Now: now, ECRImages: []zombie.ECRImage{
		{Digest: "sha256:aaaa111122223333", Repository: "web", SizeBytes: 500 << 20,
			PushedAt: daysAgo(400)},
		{Digest: "sha256:bbbb111122223333", Repository: "web", SizeBytes: 500 << 20,
			PushedAt: daysAgo(400), LastPulled: &longAgo},
		{Digest: "sha256:cccc111122223333", Repository: "web", SizeBytes: 500 << 20,
			PushedAt: daysAgo(400), LastPulled: &recent},
		{Digest: "sha256:dddd111122223333", Repository: "web", SizeBytes: 500 << 20,
			PushedAt: daysAgo(400), Tags: []string{"v1.2.3"}},
		{Digest: "sha256:eeee111122223333", Repository: "web", SizeBytes: 500 << 20,
			PushedAt: daysAgo(5)},
	}}

	got := detect.Run(inv, detect.Defaults(), []string{"ecr-untagged"}, nil)
	if len(got) != 3 {
		t.Fatalf("got %v, want the three old untagged images", ids(got))
	}

	byDigest := map[string]zombie.Finding{}
	for _, f := range got {
		byDigest[f.Metadata["digest"]] = f
	}

	if c := byDigest["sha256:aaaa111122223333"].Confidence; c != zombie.Low {
		t.Errorf("missing pull time gave %v, want LOW", c)
	}
	if c := byDigest["sha256:bbbb111122223333"].Confidence; c != zombie.Medium {
		t.Errorf("old recorded pull gave %v, want MEDIUM", c)
	}
	if c := byDigest["sha256:cccc111122223333"].Confidence; c != zombie.Low {
		t.Errorf("recent pull gave %v, want LOW", c)
	}

	reason := byDigest["sha256:aaaa111122223333"].Reason
	if !strings.Contains(reason, "no pull on record") || !strings.Contains(reason, "500.0 MiB") {
		t.Errorf("reason = %q", reason)
	}
}

func TestECRImageIDIsShortenedButDigestKept(t *testing.T) {
	inv := zombie.Inventory{Now: now, ECRImages: []zombie.ECRImage{
		{Digest: "sha256:0123456789abcdef0123456789abcdef", Repository: "web",
			SizeBytes: 100 << 20, PushedAt: daysAgo(400)},
	}}

	got := detect.Run(inv, detect.Defaults(), []string{"ecr-untagged"}, nil)
	if len(got) != 1 {
		t.Fatal("no finding")
	}
	if got[0].ResourceID != "sha256:0123456789ab" {
		t.Errorf("ResourceID = %q", got[0].ResourceID)
	}
	if got[0].Metadata["digest"] != "sha256:0123456789abcdef0123456789abcdef" {
		t.Errorf("full digest lost: %q", got[0].Metadata["digest"])
	}
}

func TestECRPriceIsExactBytesAndAnUpperBound(t *testing.T) {
	f := zombie.Finding{ResourceType: "ecr-image"}
	f.Meta("size_bytes", "536870912")

	out := price.Apply([]zombie.Finding{f}, "us-east-1")

	if out[0].MonthlyCost != 0.05 {
		t.Errorf("cost = %v, want $0.05 for half a GiB", out[0].MonthlyCost)
	}
	if out[0].Metadata["price_upper_bound"] != "true" ||
		!strings.Contains(out[0].CostBasis, "shared layers") {
		t.Errorf("not labelled an upper bound: %q", out[0].CostBasis)
	}
}

func TestECRSkipsRepositoriesItCannotRead(t *testing.T) {
	api := &fake.ECR{
		DescribeRepositoriesFunc: func(context.Context, *ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error) {
			return &ecr.DescribeRepositoriesOutput{Repositories: []ecrtypes.Repository{
				{RepositoryName: aws.String("web")},
				{RepositoryName: aws.String("locked")},
			}}, nil
		},
		DescribeImagesFunc: func(_ context.Context, in *ecr.DescribeImagesInput) (*ecr.DescribeImagesOutput, error) {
			if aws.ToString(in.RepositoryName) == "locked" {
				return nil, apiErr{code: "AccessDeniedException"}
			}
			return &ecr.DescribeImagesOutput{ImageDetails: []ecrtypes.ImageDetail{
				{ImageDigest: aws.String("sha256:aaaa"), ImageSizeInBytes: aws.Int64(500<<20 + 1234)},
			}}, nil
		},
	}

	got, err := collect.ECRImages(context.Background(), api)

	if len(got) != 1 || got[0].Repository != "web" || got[0].SizeBytes != 500<<20+1234 {
		t.Fatalf("got %+v, want the one image from web", got)
	}
	var partial *collect.Partial
	if !errors.As(err, &partial) || partial.Skipped != 1 || partial.Total != 2 {
		t.Fatalf("err = %v, want a Partial for 1 of 2 repositories", err)
	}
}

func TestECRListFailureIsNotPartial(t *testing.T) {
	api := &fake.ECR{
		DescribeRepositoriesFunc: func(context.Context, *ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error) {
			return nil, apiErr{code: "AccessDeniedException"}
		},
	}
	_, err := collect.ECRImages(context.Background(), api)

	var partial *collect.Partial
	if err == nil || errors.As(err, &partial) {
		t.Fatalf("err = %v, want a plain failure", err)
	}
}
