package price

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

//go:embed rates.json
var ratesJSON []byte

// 8760 / 12 -> average month
const HoursPerMonth = 730.0

// fallback to gp2 if volume type unkonw in findings metadata
const fallbackVolumeType = "gp2"

// mirros rates.json
type table struct {
	Updated string `json:"_updated"`
	Source  string `json:"_source"`

	EBSPerGiBMonth      map[string]float64 `json:"ebs_per_gib_month"`
	SnapshotPerGiBMonth float64            `json:"snapshot_per_gib_month"`
	ElasticIPMonth      float64            `json:"elastic_ip_month"`
	NATGatewayMonth     float64            `json:"nat_gateway_month"`
	ALBMonth            float64            `json:"alb_month"`

	RegionMultipliers       map[string]float64 `json:"region_multipliers"`
	DefaultRegionMultiplier float64            `json:"default_region_multiplier"`
}

var base table

func init() {
	// rates.json is embedded at build time to reatesJSON
	if err := json.Unmarshal(ratesJSON, &base); err != nil {
		panic("prices: rates.json is invalid: " + err.Error())
	}
}

// Data price table aws last reviewed
func Updated() string {
	return base.Updated
}

// Rates -> prie table resolved for one region
type Rates struct {
	EBSPerGiBMonth      map[string]float64
	SnapshotPerGiBMonth float64
	ElasticIPMonth      float64
	NATGatewayMonth     float64
	ALBMonth            float64

	Region           string
	RegionMultiplier float64
}

// Returns Rates for a region
func For(region string) Rates {
	mult, ok := base.RegionMultipliers[region]
	if !ok {
		mult = base.DefaultRegionMultiplier
	}
	return Rates{
		EBSPerGiBMonth:      base.EBSPerGiBMonth,
		SnapshotPerGiBMonth: base.SnapshotPerGiBMonth,
		ElasticIPMonth:      base.ElasticIPMonth,
		NATGatewayMonth:     base.NATGatewayMonth,
		ALBMonth:            base.ALBMonth,
		Region:              region,
		RegionMultiplier:    mult,
	}
}

type Pricer func(f *zombie.Finding, r Rates)

var pricers = map[string]Pricer{
	"ebs-volume":   priceEBSVolume,
	"elastic-ip":   priceElasticIP,
	"ebs-snapshot": priceSnapshot,
	"ec2-instance": priceStoppedInstance,
}

// Prices every finding for one region
func Apply(fs []zombie.Finding, region string) []zombie.Finding {
	r := For(region)

	for i := range fs {
		p, ok := pricers[fs[i].ResourceType]
		if !ok {
			fs[i].CostBasis = "no price model for " + fs[i].ResourceType
			continue
		}
		p(&fs[i], r)
	}

	return fs
}

func priceEBSVolume(f *zombie.Finding, r Rates) {
	sizeGiB, err := strconv.Atoi(f.Metadata["size_gib"])
	if err != nil || sizeGiB <= 0 {
		f.CostBasis = "unknown volume size not priced"
		return
	}

	volType := f.Metadata["volume_type"]
	rate, known := r.EBSPerGiBMonth[volType]
	if !known {
		rate = r.EBSPerGiBMonth[fallbackVolumeType]
		f.Meta("price_fallback", "true")
	}

	f.MonthlyCost = float64(sizeGiB) * rate * r.RegionMultiplier
	f.CostBasis = fmt.Sprintf("%d GiB $%.3f/GiB-mo %.2f (%s)", sizeGiB, rate, r.RegionMultiplier, r.Region)

	if !known {
		f.CostBasis += fmt.Sprintf(" [%q unknown priced as %s]", volType, fallbackVolumeType)
	}
}

// Elastic IPs bill a flat hourly rate for existing, so there is nothing to
// measure - the price is the same for every unassociated address.
func priceElasticIP(f *zombie.Finding, r Rates) {
	f.MonthlyCost = r.ElasticIPMonth * r.RegionMultiplier
	f.CostBasis = fmt.Sprintf("$%.2f/mo x %.2f (%s)",
		r.ElasticIPMonth, r.RegionMultiplier, r.Region)
}

// Snapshots bill incrementally
// Full size * rate over-estimates, -> precision lack mentioned
func priceSnapshot(f *zombie.Finding, r Rates) {
	sizeGiB, err := strconv.Atoi(f.Metadata["size_gib"])
	if err != nil || sizeGiB <= 0 {
		f.CostBasis = "unknown snapshot size not priced"
		return
	}

	f.MonthlyCost = float64(sizeGiB) * r.SnapshotPerGiBMonth * r.RegionMultiplier
	f.CostBasis = fmt.Sprintf("%d GiB * $%.3f/GiB-mo %.2f (%s) [upper bound: snapshots bill incrementally]",
		sizeGiB, r.SnapshotPerGiBMonth, r.RegionMultiplier, r.Region)
	f.Meta("price_upper_bound", "true")
}

// A stopped instance's compute is free, the cost is its attached volumes.
func priceStoppedInstance(f *zombie.Finding, r Rates) {
	rawSizes := f.Metadata["volume_sizes_gib"]
	rawTypes := f.Metadata["volume_types"]
	if rawSizes == "" || rawTypes == "" {
		f.CostBasis = "unknown volume sizes not priced"
		return
	}

	sizes := strings.Split(rawSizes, ",")
	types := strings.Split(rawTypes, ",")
	if len(sizes) != len(types) {
		f.CostBasis = "mismatched volume metadata not priced"
		return
	}

	var total float64
	parts := make([]string, 0, len(sizes))
	for i := range sizes {
		gib, err := strconv.Atoi(sizes[i])
		if err != nil || gib <= 0 {
			continue
		}
		rate, known := r.EBSPerGiBMonth[types[i]]
		if !known {
			rate = r.EBSPerGiBMonth[fallbackVolumeType]
			f.Meta("price_fallback", "true")
		}
		total += float64(gib) * rate * r.RegionMultiplier
		parts = append(parts, fmt.Sprintf("%d GiB %s", gib, types[i]))
	}

	f.MonthlyCost = total
	f.CostBasis = fmt.Sprintf("%s x %.2f (%s), attached volumes only - compute is free",
		strings.Join(parts, " + "), r.RegionMultiplier, r.Region)
}
