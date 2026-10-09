package collect

import (
	"context"
	"fmt"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
	"github.com/aws/aws-sdk-go-v2/service/efs"
)

func Fileystems(ctx context.Context, api awsapi.EFSAPI) ([]zombie.FileSystem, error) {
	out := []zombie.FileSystem{}

	p := efs.NewDescribeFileSystemsPaginator(api, &efs.DescribeFileSystemsInput{})

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("elasticfilesystem:DescribeFileSystems: %w", err)
		}
		for _, fs := range page.FileSystems {
			out = append(out, toFileSystem(fs))
		}
	}
	return out, nil
}
