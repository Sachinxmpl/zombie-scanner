package collect

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// Returns every image in every repository, one DescribeImages walk per repo.
// A repository that cannot be read is skipped and counted.
func ECRImages(ctx context.Context, api awsapi.ECRAPI) ([]zombie.ECRImage, error) {
	out := []zombie.ECRImage{}
	total, skipped := 0, 0

	repos := ecr.NewDescribeRepositoriesPaginator(api, &ecr.DescribeRepositoriesInput{})

	for repos.HasMorePages() {
		page, err := repos.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ecr:DescribeRepositories: %w", err)
		}

		for _, r := range page.Repositories {
			name := aws.ToString(r.RepositoryName)
			if name == "" {
				continue
			}
			total++
			arn := aws.ToString(r.RepositoryArn)

			images := ecr.NewDescribeImagesPaginator(api, &ecr.DescribeImagesInput{
				RepositoryName: &name,
			})
			for images.HasMorePages() {
				ip, err := images.NextPage(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return nil, fmt.Errorf("ecr:DescribeImages: %w", err)
					}
					skipped++
					break
				}
				for _, d := range ip.ImageDetails {
					out = append(out, toECRImage(d, name, arn))
				}
			}
		}
	}

	if skipped > 0 {
		return out, &Partial{Service: "ecr", Operation: "DescribeImages",
			Skipped: skipped, Total: total, What: "repositories",
			Hint: "a repository policy can deny this role even when IAM allows it"}
	}
	return out, nil
}
