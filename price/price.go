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

// RDS storage types span $0.100-$0.125/GiB-mo, so a fallback is wrong by at most 25%.
const fallbackRDSStorageType = "gp2"

// mirros rates.json
type table struct {
	Updated string `json:"_updated"`
	Source  string `json:"_source"`

	EBSPerGiBMonth        map[string]float64 `json:"ebs_per_gib_month"`
	RDSStoragePerGiBMonth map[string]float64 `json:"rds_storage_per_gib_month"`
	RDSInstanceHour       map[string]float64 `json:"rds_instance_hour"`
	SnapshotPerGiBMonth   float64            `json:"snapshot_per_gib_month"`
	ElasticIPMonth        float64            `json:"elastic_ip_month"`
	NATGatewayMonth       float64            `json:"nat_gateway_month"`
	ALBMonth              float64            `json:"alb_month"`
	EFSPerGiBMonth        map[string]float64 `json:"efs_per_gib_month"`

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
	EBSPerGiBMonth        map[string]float64
	RDSStoragePerGiBMonth map[string]float64
	RDSInstanceHour       map[string]float64
	SnapshotPerGiBMonth   float64
	ElasticIPMonth        float64
	NATGatewayMonth       float64
	ALBMonth              float64
	EFSPerGiBMonth        map[string]float64

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
		EBSPerGiBMonth:        base.EBSPerGiBMonth,
		RDSStoragePerGiBMonth: base.RDSStoragePerGiBMonth,
		RDSInstanceHour:       base.RDSInstanceHour,
		SnapshotPerGiBMonth:   base.SnapshotPerGiBMonth,
		ElasticIPMonth:        base.ElasticIPMonth,
		NATGatewayMonth:       base.NATGatewayMonth,
		ALBMonth:              base.ALBMonth,
		EFSPerGiBMonth:        base.EFSPerGiBMonth,
		Region:                region,
		RegionMultiplier:      mult,
	}
}

type Pricer func(f *zombie.Finding, r Rates)

var pricers = map[string]Pricer{
	"ebs-volume":     priceEBSVolume,
	"elastic-ip":     priceElasticIP,
	"ebs-snapshot":   priceSnapshot,
	"ec2-instance":   priceStoppedInstance,
	"nat-gateway":    priceNATGateway,
	"alb":            priceALB,
	"rds-instance":   priceRDSInstance,
	"efs-filesystem": priceEFS,
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

// An RDS instance bills for storage always, and for instance-hours only while it is running
func priceRDSInstance(f *zombie.Finding, r Rates) {
	storage, basis := rdsStorageCost(f, r)
	if basis == "" {
		f.CostBasis = "unknown storage size not priced"
		return
	}

	total, parts := storage, basis
	if f.Metadata["status"] == "available" {
		compute, note := rdsComputeCost(f, r)
		total += compute
		parts += " + " + note
	} else {
		parts += " (compute is free while stopped)"
	}

	az := "single-AZ"
	if rdsAZMultiplier(f) == 2 {
		az = "multi-AZ x2"
	}

	f.MonthlyCost = total
	f.CostBasis = fmt.Sprintf("%s, %s x %.2f (%s) [lower bound: backups excluded]",
		parts, az, r.RegionMultiplier, r.Region)
	f.Meta("price_lower_bound", "true")
}

func rdsStorageCost(f *zombie.Finding, r Rates) (float64, string) {
	gib, err := strconv.Atoi(f.Metadata["storage_gib"])
	if err != nil || gib <= 0 {
		return 0, ""
	}

	storageType := f.Metadata["storage_type"]
	rate, known := r.RDSStoragePerGiBMonth[storageType]
	if !known {
		rate = r.RDSStoragePerGiBMonth[fallbackRDSStorageType]
		f.Meta("price_fallback", "true")
	}

	basis := fmt.Sprintf("%d GiB %s $%.3f/GiB-mo", gib, storageType, rate)
	if !known {
		basis += fmt.Sprintf(" [%q unknown priced as %s]", storageType, fallbackRDSStorageType)
	}
	return float64(gib) * rate * rdsAZMultiplier(f) * r.RegionMultiplier, basis
}

// Returns zero and a stated reason when the class or engine is not in the table.
// RDS classes span 500x, so no fallback
func rdsComputeCost(f *zombie.Finding, r Rates) (float64, string) {
	engine := f.Metadata["engine"]
	if strings.HasPrefix(engine, "oracle") || strings.HasPrefix(engine, "sqlserver") {
		f.Meta("price_partial", "true")
		return 0, fmt.Sprintf("compute not priced (%s licence cost varies by licence model)", engine)
	}

	class := f.Metadata["instance_class"]
	hourly, known := r.RDSInstanceHour[class]
	if !known {
		f.Meta("price_partial", "true")
		return 0, fmt.Sprintf("compute not priced (no rate for %s)", class)
	}

	return hourly * HoursPerMonth * rdsAZMultiplier(f) * r.RegionMultiplier,
		fmt.Sprintf("%s $%.3f/hr x %.0f hr", class, hourly, HoursPerMonth)
}

func rdsAZMultiplier(f *zombie.Finding) float64 {
	if f.Metadata["multi_az"] == "true" {
		return 2
	}
	return 1
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

// NAT gateways bill hourly for existing, before any data processing charges
func priceNATGateway(f *zombie.Finding, r Rates) {
	f.MonthlyCost = r.NATGatewayMonth * r.RegionMultiplier
	f.CostBasis = fmt.Sprintf("$%.2f/mo x %.2f (%s), hourly charge only - excludes data processing", r.NATGatewayMonth, r.RegionMultiplier, r.Region)
}

// ALBs bill hourly plus LCU-hours. For an idle one LCU usage is near zero,
// so the hourly charge is close to the whole cost.
func priceALB(f *zombie.Finding, r Rates) {
	f.MonthlyCost = r.ALBMonth * r.RegionMultiplier
	f.CostBasis = fmt.Sprintf("$%.2f/mo x %.2f (%s), hourly charge only - excludes LCU",
		r.ALBMonth, r.RegionMultiplier, r.Region)
}

func priceEFS(f *zombie.Finding, r Rates) {
	var total float64
	parts := make([]string, 0, 3)

	for _, tier := range []string{"standard", "ia", "archive"} {
		gib, err := strconv.Atoi(f.Metadata[tier+"_gib"])
		if err != nil || gib <= 0 {
			continue
		}
		rate, known := r.EFSPerGiBMonth[tier]
		if !known {
			continue
		}
		total += float64(gib) * rate * r.RegionMultiplier
		parts = append(parts, fmt.Sprintf("%d GiB %s $%.3f/GiB-mo", gib, tier, rate))
	}

	if len(parts) == 0 {
		f.CostBasis = "unknown file system size not priced"
		return
	}

	f.MonthlyCost = total
	f.CostBasis = fmt.Sprintf("%s x %.2f (%s)",
		strings.Join(parts, " + "), r.RegionMultiplier, r.Region)
}
