package test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/awsapi/fake"
	"github.com/Sachinxmpl/zombie-scanner/collect"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/filter"
	"github.com/Sachinxmpl/zombie-scanner/scan"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// repositories "kept" and "plain", two old untagged images each; tags are whatever tagsFor returns
func ecrWithRepoTags(tagsFor func(repo string) ([]ecrtypes.Tag, error)) *fake.ECR {
	return &fake.ECR{
		DescribeRepositoriesFunc: func(context.Context, *ecr.DescribeRepositoriesInput) (*ecr.DescribeRepositoriesOutput, error) {
			return &ecr.DescribeRepositoriesOutput{Repositories: []ecrtypes.Repository{
				{RepositoryName: aws.String("kept"), RepositoryArn: aws.String("arn:aws:ecr:us-east-1:123456789012:repository/kept")},
				{RepositoryName: aws.String("plain"), RepositoryArn: aws.String("arn:aws:ecr:us-east-1:123456789012:repository/plain")},
			}}, nil
		},
		DescribeImagesFunc: func(_ context.Context, in *ecr.DescribeImagesInput) (*ecr.DescribeImagesOutput, error) {
			repo := aws.ToString(in.RepositoryName)
			return &ecr.DescribeImagesOutput{ImageDetails: []ecrtypes.ImageDetail{
				{ImageDigest: aws.String("sha256:" + repo + "1"), ImageSizeInBytes: aws.Int64(1 << 30), ImagePushedAt: aws.Time(daysAgo(400))},
				{ImageDigest: aws.String("sha256:" + repo + "2"), ImageSizeInBytes: aws.Int64(1 << 30), ImagePushedAt: aws.Time(daysAgo(400))},
			}}, nil
		},
		ListTagsForResourceFunc: func(_ context.Context, in *ecr.ListTagsForResourceInput) (*ecr.ListTagsForResourceOutput, error) {
			arn := aws.ToString(in.ResourceArn)
			tags, err := tagsFor(arn[strings.LastIndex(arn, "/")+1:])
			if err != nil {
				return nil, err
			}
			return &ecr.ListTagsForResourceOutput{Tags: tags}, nil
		},
	}
}

func keepTagOnKeptRepo(repo string) ([]ecrtypes.Tag, error) {
	if repo == "kept" {
		return []ecrtypes.Tag{{Key: aws.String(filter.DefaultKeepTag), Value: aws.String("base images")}}, nil
	}
	return nil, nil
}

func ecrScan(t *testing.T, api *fake.ECR) zombie.Report {
	t.Helper()
	eng := &scan.Engine{
		Accounts: []awsapi.Factory{&fake.Factory{
			Clients: awsapi.Clients{EC2: &fake.EC2{}, CW: &fake.CloudWatch{}, ELB: &fake.ELB{},
				RDS: &fake.RDS{}, EFS: &fake.EFS{}, KMS: &fake.KMS{}, ECR: api},
			Base: "us-east-1",
		}},
		Cfg:     detect.Defaults(),
		Filters: []filter.Filter{filter.KeepTag{Key: filter.DefaultKeepTag}},
		Clock:   func() time.Time { return now },
	}
	rep, err := eng.Run(context.Background(), scan.Options{Only: []string{"ecr-untagged"}})
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func repos(fs []zombie.Finding) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, f.Metadata["repository"])
	}
	return out
}

func TestECRKeepTagOnRepositoryHidesItsImages(t *testing.T) {
	api := ecrWithRepoTags(keepTagOnKeptRepo)
	rep := ecrScan(t, api)

	if !slices.Equal(repos(rep.Findings), []string{"plain", "plain"}) {
		t.Errorf("findings from %v, want both images from plain only", repos(rep.Findings))
	}
	if len(rep.Errors) != 0 {
		t.Errorf("errors = %+v, want none", rep.Errors)
	}
	// once per repository, not once per image
	asked := 0
	for _, c := range api.Calls() {
		if c == "ListTagsForResource" {
			asked++
		}
	}
	if asked != 2 {
		t.Errorf("ListTagsForResource called %d times, want 2 (one per repository)", asked)
	}
}

// one repository policy denies the tags: its images are skipped
func TestECRRepositoryWithUnreadableTagsIsSkipped(t *testing.T) {
	rep := ecrScan(t, ecrWithRepoTags(func(repo string) ([]ecrtypes.Tag, error) {
		if repo == "kept" {
			return nil, apiErr{code: "AccessDeniedException"}
		}
		return nil, nil
	}))

	if !slices.Equal(repos(rep.Findings), []string{"plain", "plain"}) {
		t.Errorf("findings from %v, want both images from plain only", repos(rep.Findings))
	}
	if len(rep.Errors) != 1 || rep.Errors[0].Operation != "ListTagsForResource" {
		t.Fatalf("errors = %+v, want one ListTagsForResource warning", rep.Errors)
	}
	if rep.Errors[0].Kind == zombie.KindAccessDenied {
		t.Errorf("one repository's policy said no; reported as a missing IAM permission: %+v", rep.Errors[0])
	}
}

// the permission is missing: no image can be shown, and the report names the permission
func TestECRRepoTagsDeniedEverywhere(t *testing.T) {
	rep := ecrScan(t, ecrWithRepoTags(func(string) ([]ecrtypes.Tag, error) {
		return nil, apiErr{code: "AccessDeniedException"}
	}))

	if len(rep.Findings) != 0 {
		t.Errorf("findings from %v, want none", repos(rep.Findings))
	}
	if len(rep.Errors) != 1 || rep.Errors[0].Kind != zombie.KindAccessDenied ||
		!slices.Equal(rep.Errors[0].Affects, []string{"ecr-untagged"}) {
		t.Errorf("errors = %+v, want one missing permission affecting ecr-untagged", rep.Errors)
	}
}

func TestECRRepoTagsPartialCounts(t *testing.T) {
	images := []zombie.ECRImage{
		{Digest: "a1", RepoARN: "arn:aws:ecr:us-east-1:1:repository/a"},
		{Digest: "a2", RepoARN: "arn:aws:ecr:us-east-1:1:repository/a"},
		{Digest: "b1", RepoARN: "arn:aws:ecr:us-east-1:1:repository/b"},
	}
	err := collect.ECRRepoTags(context.Background(), ecrWithRepoTags(func(repo string) ([]ecrtypes.Tag, error) {
		if repo == "a" {
			return nil, apiErr{code: "AccessDeniedException"}
		}
		return keepTagOnKeptRepo("kept")
	}), images)

	var partial *collect.Partial
	if !errors.As(err, &partial) || partial.Skipped != 1 || partial.Total != 2 {
		t.Fatalf("err = %v, want a Partial for 1 of 2 repositories", err)
	}
	if !images[0].RepoTagsUnknown || !images[1].RepoTagsUnknown || images[2].RepoTagsUnknown {
		t.Errorf("RepoTagsUnknown = %v %v %v, want true true false",
			images[0].RepoTagsUnknown, images[1].RepoTagsUnknown, images[2].RepoTagsUnknown)
	}
	if _, ok := images[2].RepoTags[filter.DefaultKeepTag]; !ok {
		t.Errorf("RepoTags = %v, want the keep tag copied onto the image", images[2].RepoTags)
	}
}
