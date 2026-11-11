package collect

import (
	"context"
	"fmt"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// Returns every KMS key with its metadata. ListKeys gives only ids, so this costs one DescribeKey per key.
// A key that cannot be described is skippedmand counted, so one restrictive key policy does not hide all the others.
func KMSKeys(ctx context.Context, api awsapi.KMSAPI) ([]zombie.KMSKey, error) {
	out := []zombie.KMSKey{}
	total, skipped := 0, 0

	p := kms.NewListKeysPaginator(api, &kms.ListKeysInput{})

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("kms:ListKeys: %w", err)
		}
		for _, k := range page.Keys {
			id := aws.ToString(k.KeyId)
			if id == "" {
				continue
			}
			total++
			resp, err := api.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: &id})
			if err != nil {
				// a cancelled scan fails every key, which is not a key problem
				if ctx.Err() != nil {
					return nil, fmt.Errorf("kms:DescribeKey: %w", err)
				}
				skipped++
				continue
			}
			if resp.KeyMetadata != nil {
				out = append(out, toKMSKey(*resp.KeyMetadata))
			}
		}
	}

	if skipped > 0 {
		return out, &Partial{Service: "kms", Operation: "DescribeKey",
			Skipped: skipped, Total: total, What: "keys",
			Hint: "a key policy can deny this role even when IAM allows it"}
	}
	return out, nil
}

// Fills in Tags on customer-managed keys
func KMSKeyTags(ctx context.Context, api awsapi.KMSAPI, keys []zombie.KMSKey) error {
	total, skipped := 0, 0
	var lastErr error

	for i := range keys {
		if keys[i].Manager != "CUSTOMER" {
			continue
		}
		total++
		tags, err := kmsTags(ctx, api, keys[i].ID)
		if err != nil {
			keys[i].TagsUnknown = true
			skipped++
			lastErr = err
			continue
		}
		keys[i].Tags = tags
	}

	if skipped == 0 {
		return nil
	}

	// every key failed, most likely IAM
	if skipped == total {
		return fmt.Errorf("kms:ListResourceTags: %w", lastErr)
	}
	return &Partial{Service: "kms", Operation: "ListResourceTags",
		Skipped: skipped, Total: total, What: "key tags",
		Hint: "a key policy can deny this role even when IAM allows it"}
}

func kmsTags(ctx context.Context, api awsapi.KMSAPI, id string) (map[string]string, error) {
	m := map[string]string{}
	p := kms.NewListResourceTagsPaginator(api, &kms.ListResourceTagsInput{KeyId: &id})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range page.Tags {
			if k := aws.ToString(t.TagKey); k != "" {
				m[k] = aws.ToString(t.TagValue)
			}
		}
	}
	return m, nil
}
