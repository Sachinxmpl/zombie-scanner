// turns AWS API responses into a zombie.Inventory
package collect

import (
	"time"

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

func toSnapshot(s ec2types.Snapshot) zombie.Snapshot {
	return zombie.Snapshot{
		ID:          aws.ToString(s.SnapshotId),
		VolumeID:    aws.ToString(s.VolumeId),
		SizeGiB:     aws.ToInt32(s.VolumeSize),
		StartedAt:   aws.ToTime(s.StartTime),
		Description: aws.ToString(s.Description),
		Tags:        toTags(s.Tags),
	}
}

func toImage(i ec2types.Image) zombie.Image {
	out := zombie.Image{
		ID:        aws.ToString(i.ImageId),
		Name:      aws.ToString(i.Name),
		CreatedAt: parseImageDate(aws.ToString(i.CreationDate)),
	}

	for _, bdm := range i.BlockDeviceMappings {
		if bdm.Ebs == nil {
			continue
		}
		if id := aws.ToString(bdm.Ebs.SnapshotId); id != "" {
			out.SnapshotIDs = append(out.SnapshotIDs, id)
		}
	}
	return out
}

func parseImageDate(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func toInstance(i ec2types.Instance) zombie.Instance {
	out := zombie.Instance{
		ID:                    aws.ToString(i.InstanceId),
		Type:                  string(i.InstanceType),
		LaunchedAt:            aws.ToTime(i.LaunchTime),
		StateTransitionReason: aws.ToString(i.StateTransitionReason),
		Tags:                  toTags(i.Tags),
	}
	if i.State != nil {
		out.State = string(i.State.Name)
	}
	for _, bdm := range i.BlockDeviceMappings {
		if bdm.Ebs == nil {
			continue
		}
		if id := aws.ToString(bdm.Ebs.VolumeId); id != "" {
			out.VolumeIDs = append(out.VolumeIDs, id)
		}
	}
	return out
}

func toNATGateway(n ec2types.NatGateway) zombie.NATGateway {
	return zombie.NATGateway{
		ID:        aws.ToString(n.NatGatewayId),
		VPCID:     aws.ToString(n.VpcId),
		State:     string(n.State),
		CreatedAt: aws.ToTime(n.CreateTime),
		Tags:      toTags(n.Tags),
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
