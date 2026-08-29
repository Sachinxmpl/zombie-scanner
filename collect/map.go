// turns AWS API responses into a zombie.Inventory
package collect

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// converts an AWS SDK Volume to a zombie.Volume
func toVolume(v ec2types.Volume) zombie.Volume {
	out := zombie.Volume{
		ID:         aws.ToString(v.VolumeId),
		State:      string(v.State),
		VolumeType: string(v.VolumeType),
		SizeGiB:    aws.ToInt32(v.Size),
		CreatedAt:  aws.ToTime(v.CreateTime),
		Tags:       toTags(v.Tags),
	}

	for _, a := range v.Attachments {
		if id := aws.ToString(a.InstanceId); id != "" {
			out.AttachedTo = id
			break
		}
	}

	return out
}

func toAddress(a ec2types.Address) zombie.Address {
	return zombie.Address{
		AllocationID:  aws.ToString(a.AllocationId),
		PublicIP:      aws.ToString(a.PublicIp),
		AssociationID: aws.ToString(a.AssociationId),
		Tags:          toTags(a.Tags),
	}
}

func toTags(tags []ec2types.Tag) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	m := make(map[string]string, len(tags))
	for _, t := range tags {
		if K := aws.ToString(t.Key); K != "" {
			m[K] = aws.ToString(t.Value)
		}
	}
	return m
}
