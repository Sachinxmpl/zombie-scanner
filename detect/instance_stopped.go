package detect

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(instanceStopped{})
}

type instanceStopped struct{}

func (instanceStopped) Name() string {
	return "instance-stopped"
}

func (instanceStopped) Describe() string {
	return "Stopped instances whose attached EBS volumes keep billing at full price"
}

func (instanceStopped) Needs() []string {
	return []string{
		"ec2:DescribeInstances",
		"ec2:DescribeVolumes",
	}
}

func (instanceStopped) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}

	// without volume size every finding would price at $0.00
	if inv.Missing("ec2:DescribeVolumes") {
		return out
	}

	byID := make(map[string]zombie.Volume, len(inv.Volumes))
	for _, v := range inv.Volumes {
		byID[v.ID] = v
	}

	for _, i := range inv.Instances {
		if i.State != "stopped" {
			continue
		}

		var sizes, types []string
		attached := 0
		for _, id := range i.VolumeIDs {
			v, ok := byID[id]
			if !ok {
				continue
			}
			sizes = append(sizes, strconv.Itoa(int(v.SizeGiB)))
			types = append(types, v.VolumeType)
			attached++
		}

		if attached == 0 {
			continue
		}

		conf := zombie.Medium
		reason := fmt.Sprintf("Stopped, duration unknown (%s); %d attached volume(s) still billing",
			i.Type, attached)

		if stoppedAt, ok := parseStopTime(i.StateTransitionReason); ok {
			days := inv.AgeDays(stoppedAt)
			if days < cfg.StoppedDays {
				continue
			}
			reason = fmt.Sprintf("Stopped %d days ago (%s); %d attached volume(s) still billing",
				days, i.Type, attached)
		} else {
			// never invent a duration from LaunchTime, downgrade instead
			conf = zombie.Low
		}

		f := zombie.Finding{
			ResourceID:   i.ID,
			ResourceType: "ec2-instance",
			Confidence:   conf,
			Reason:       reason,
			Tags:         i.Tags,
		}
		f.Meta("instance_type", i.Type)
		f.Meta("volume_count", strconv.Itoa(attached))
		f.Meta("volume_sizes_gib", strings.Join(sizes, ","))
		f.Meta("volume_types", strings.Join(types, ","))

		out = append(out, f)
	}

	return out
}

// "User initiated (2026-03-14 09:21:15 GMT)"
var stopTimeRe = regexp.MustCompile(`\((\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) GMT\)`)

func parseStopTime(reason string) (time.Time, bool) {
	m := stopTimeRe.FindStringSubmatch(reason)
	if m == nil {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02 15:04:05", m[1])
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}
