package price

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
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

// mirrors rates.json
type Table struct {
	Updated string `json:"_updated"`
	Source  string `json:"_source"`

	// where the prices came from, also shown in the report
	Label string `json:"-"`

	EBSPerGiBMonth        map[string]float64 `json:"ebs_per_gib_month"`
	RDSStoragePerGiBMonth map[string]float64 `json:"rds_storage_per_gib_month"`
	RDSInstanceHour       map[string]float64 `json:"rds_instance_hour"`
	SnapshotPerGiBMonth   float64            `json:"snapshot_per_gib_month"`
	ElasticIPMonth        float64            `json:"elastic_ip_month"`
	NATGatewayMonth       float64            `json:"nat_gateway_month"`
	ALBMonth              float64            `json:"alb_month"`
	RDSBackupPerGiBMonth  float64            `json:"rds_backup_per_gib_month"`
	EFSPerGiBMonth        map[string]float64 `json:"efs_per_gib_month"`
	KMSKeyMonth           float64            `json:"kms_key_month"`
	ECRPerGiBMonth        float64            `json:"ecr_per_gib_month"`

	RegionMultipliers       map[string]float64 `json:"region_multipliers"`
	DefaultRegionMultiplier float64            `json:"default_region_multiplier"`
}

// read-only; anything that changes prices works on its own copy from Default
var base = Default()

// Default returns a fresh copy of the built-in prices.
func Default() *Table {
	t := &Table{}
	// rates.json is embedded at build time to ratesJSON
	if err := json.Unmarshal(ratesJSON, t); err != nil {
		panic("prices: rates.json is invalid: " + err.Error())
	}
	t.Label = "built-in " + t.Updated
	return t
}

// Load lays a user's price file over the built-in prices
// Keys it leaves out keep the built-in value ,keys starting with "_" are notes and are skipped.
func Load(path string) (*Table, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("--rates: %w", err)
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, fmt.Errorf("--rates %s: %w", path, err)
	}
	known := knownKeys()
	var unknown []string
	for k := range keys {
		if !strings.HasPrefix(k, "_") && !known[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("--rates %s: unknown key(s) %s", path, strings.Join(unknown, ", "))
	}

	t := Default()
	if err := json.Unmarshal(raw, t); err != nil {
		return nil, fmt.Errorf("--rates %s: %w", path, err)
	}
	if err := t.validate(); err != nil {
		return nil, fmt.Errorf("--rates %s: %w", path, err)
	}
	t.Label = "custom: " + filepath.Base(path)
	return t, nil
}

func knownKeys() map[string]bool {
	raw, _ := json.Marshal(base)
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

func (t *Table) validate() error {
	single := map[string]float64{
		"snapshot_per_gib_month":    t.SnapshotPerGiBMonth,
		"elastic_ip_month":          t.ElasticIPMonth,
		"nat_gateway_month":         t.NATGatewayMonth,
		"alb_month":                 t.ALBMonth,
		"rds_backup_per_gib_month":  t.RDSBackupPerGiBMonth,
		"kms_key_month":             t.KMSKeyMonth,
		"ecr_per_gib_month":         t.ECRPerGiBMonth,
		"default_region_multiplier": t.DefaultRegionMultiplier,
	}
	maps := map[string]map[string]float64{
		"ebs_per_gib_month":         t.EBSPerGiBMonth,
		"rds_storage_per_gib_month": t.RDSStoragePerGiBMonth,
		"rds_instance_hour":         t.RDSInstanceHour,
		"efs_per_gib_month":         t.EFSPerGiBMonth,
		"region_multipliers":        t.RegionMultipliers,
	}

	var bad []string
	for name, v := range single {
		if v < 0 {
			bad = append(bad, name)
		}
	}
	for name, m := range maps {
		if m == nil {
			return fmt.Errorf("%s cannot be null", name)
		}
		for k, v := range m {
			if v < 0 {
				bad = append(bad, name+"."+k)
			}
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return errors.New("negative price(s): " + strings.Join(bad, ", "))
	}
	return nil
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
	RDSBackupPerGiBMonth  float64
	EFSPerGiBMonth        map[string]float64
	KMSKeyMonth           float64
	ECRPerGiBMonth        float64

	Region           string
	RegionMultiplier float64
}

// Returns Rates for a region from the built-in prices
func For(region string) Rates {
	return base.For(region)
}

func (t *Table) For(region string) Rates {
	mult, ok := t.RegionMultipliers[region]
	if !ok {
		mult = t.DefaultRegionMultiplier
	}
	return Rates{
		EBSPerGiBMonth:        t.EBSPerGiBMonth,
		RDSStoragePerGiBMonth: t.RDSStoragePerGiBMonth,
		RDSInstanceHour:       t.RDSInstanceHour,
		SnapshotPerGiBMonth:   t.SnapshotPerGiBMonth,
		ElasticIPMonth:        t.ElasticIPMonth,
		NATGatewayMonth:       t.NATGatewayMonth,
		ALBMonth:              t.ALBMonth,
		RDSBackupPerGiBMonth:  t.RDSBackupPerGiBMonth,
		EFSPerGiBMonth:        t.EFSPerGiBMonth,
		KMSKeyMonth:           t.KMSKeyMonth,
		ECRPerGiBMonth:        t.ECRPerGiBMonth,
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
	"rds-snapshot":   priceRDSSnapshot,
	"efs-filesystem": priceEFS,
	"kms-key":        priceKMSKey,
	"ecr-image":      priceECRImage,
}

// rounds to whole cents so the report holds 1.64, not 1.6400000000000001
func Cents(usd float64) float64 {
	return math.Round(usd*100) / 100
}

// Prices every finding for one region with the built-in prices
func Apply(fs []zombie.Finding, region string) []zombie.Finding {
	return base.Apply(fs, region)
}

func (t *Table) Apply(fs []zombie.Finding, region string) []zombie.Finding {
	r := t.For(region)

	for i := range fs {
		p, ok := pricers[fs[i].ResourceType]
		if !ok {
			fs[i].CostBasis = "no price model for " + fs[i].ResourceType
			continue
		}
		p(&fs[i], r)
		fs[i].MonthlyCost = Cents(fs[i].MonthlyCost)
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

// A stopped instance's compute is free, the cost is its attached volumes and Elastic IPs.
func priceStoppedInstance(f *zombie.Finding, r Rates) {
	rawSizes := f.Metadata["volume_sizes_gib"]
	rawTypes := f.Metadata["volume_types"]
	rawIPs := f.Metadata["elastic_ips"]
	if (rawSizes == "" || rawTypes == "") && rawIPs == "" {
		f.CostBasis = "unknown volume sizes not priced"
		return
	}

	var sizes, types []string
	if rawSizes != "" && rawTypes != "" {
		sizes = strings.Split(rawSizes, ",")
		types = strings.Split(rawTypes, ",")
	}
	if len(sizes) != len(types) {
		f.CostBasis = "mismatched volume metadata not priced"
		return
	}

	var total float64
	parts := make([]string, 0, len(sizes)+1)
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

	if rawIPs != "" {
		n := len(strings.Split(rawIPs, ","))
		total += float64(n) * r.ElasticIPMonth * r.RegionMultiplier
		parts = append(parts, fmt.Sprintf("%d Elastic IP x $%.2f/mo", n, r.ElasticIPMonth))
	}

	f.MonthlyCost = total
	f.CostBasis = fmt.Sprintf("%s x %.2f (%s), compute is free",
		strings.Join(parts, " + "), r.RegionMultiplier, r.Region)
	if f.Metadata["elastic_ips_unknown"] == "true" {
		f.CostBasis += " [Elastic IPs not checked]"
	}
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

// Backup storage bills on what the snapshot occupies. When AWS did not report
// that, the instance's allocated storage is an upper bound.
func priceRDSSnapshot(f *zombie.Finding, r Rates) {
	bytes, err := strconv.ParseInt(f.Metadata["size_bytes"], 10, 64)
	if err != nil || bytes <= 0 {
		f.CostBasis = "unknown snapshot size not priced"
		return
	}
	gib := float64(bytes) / (1 << 30)

	f.MonthlyCost = gib * r.RDSBackupPerGiBMonth * r.RegionMultiplier
	f.CostBasis = fmt.Sprintf("%.2f GiB $%.3f/GiB-mo x %.2f (%s)",
		gib, r.RDSBackupPerGiBMonth, r.RegionMultiplier, r.Region)

	if f.Metadata["size_is_instance_allocation"] == "true" {
		f.CostBasis += " [upper bound: snapshot size unreported, using the instance allocation]"
		f.Meta("price_upper_bound", "true")
	}
}

// A flat fee per key, the same in every commercial region
func priceKMSKey(f *zombie.Finding, r Rates) {
	f.MonthlyCost = r.KMSKeyMonth
	f.CostBasis = fmt.Sprintf("$%.2f/mo per customer-managed key (%s), excludes API request charges",
		r.KMSKeyMonth, r.Region)
}

func priceECRImage(f *zombie.Finding, r Rates) {
	bytes, err := strconv.ParseInt(f.Metadata["size_bytes"], 10, 64)
	if err != nil || bytes <= 0 {
		f.CostBasis = "unknown image size not priced"
		return
	}
	gib := float64(bytes) / (1 << 30)

	f.MonthlyCost = gib * r.ECRPerGiBMonth * r.RegionMultiplier
	f.CostBasis = fmt.Sprintf("%.2f GiB $%.2f/GiB-mo x %.2f (%s) [upper bound: shared layers are stored once]",
		gib, r.ECRPerGiBMonth, r.RegionMultiplier, r.Region)
	f.Meta("price_upper_bound", "true")
}

func priceEFS(f *zombie.Finding, r Rates) {
	var total float64
	parts := make([]string, 0, 3)

	for _, tier := range []string{"standard", "ia", "archive"} {
		bytes, err := strconv.ParseInt(f.Metadata[tier+"_bytes"], 10, 64)
		if err != nil || bytes <= 0 {
			continue
		}
		gib := float64(bytes) / (1 << 30)
		rate, known := r.EFSPerGiBMonth[tier]
		if !known {
			continue
		}
		total += gib * rate * r.RegionMultiplier
		parts = append(parts, fmt.Sprintf("%.2f GiB %s $%.3f/GiB-mo", gib, tier, rate))
	}

	if len(parts) == 0 {
		f.CostBasis = "unknown file system size not priced"
		return
	}

	f.MonthlyCost = total
	f.CostBasis = fmt.Sprintf("%s x %.2f (%s)",
		strings.Join(parts, " + "), r.RegionMultiplier, r.Region)
}
