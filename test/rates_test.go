package test

import (
	"bytes"
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/awsapi/fake"
	"github.com/Sachinxmpl/zombie-scanner/cli"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/price"
	"github.com/Sachinxmpl/zombie-scanner/render"
	"github.com/Sachinxmpl/zombie-scanner/scan"
)

func ratesFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "my-prices.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.0001 }

func fakeUnassociatedAddress() *fake.EC2 {
	return &fake.EC2{
		DescribeAddressesFunc: func(context.Context, *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error) {
			return &ec2.DescribeAddressesOutput{Addresses: []ec2types.Address{{
				AllocationId: aws.String("eipalloc-1"),
				PublicIp:     aws.String("203.0.113.9"),
			}}}, nil
		},
	}
}

// Users list only the prices that differ; everything else keeps the built-in value.
func TestRatesFileOverridesOnlyItsKeys(t *testing.T) {
	tbl, err := price.Load(ratesFile(t, `{
		"_notes": "our EDP discount",
		"elastic_ip_month": 1,
		"ebs_per_gib_month": {"gp3": 0.05}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	def := price.Default()
	r := tbl.For("us-east-1")

	for _, c := range []struct {
		name      string
		got, want float64
	}{
		{"elastic ip (set)", r.ElasticIPMonth, 1},
		{"gp3 (set)", r.EBSPerGiBMonth["gp3"], 0.05},
		{"gp2 (same map, not set)", r.EBSPerGiBMonth["gp2"], def.EBSPerGiBMonth["gp2"]},
		{"nat (not set)", r.NATGatewayMonth, def.NATGatewayMonth},
		{"region multiplier (not set)", tbl.For("eu-central-1").RegionMultiplier, def.For("eu-central-1").RegionMultiplier},
	} {
		if !near(c.got, c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if !strings.HasPrefix(tbl.Label, "custom: my-prices.json") {
		t.Errorf("Label = %q", tbl.Label)
	}
}

// Loading a custom file must not leak into the built-in prices other code uses.
func TestRatesFileLeavesBuiltInPricesAlone(t *testing.T) {
	before := price.For("us-east-1").EBSPerGiBMonth["gp3"]
	if _, err := price.Load(ratesFile(t, `{"ebs_per_gib_month": {"gp3": 0.01}}`)); err != nil {
		t.Fatal(err)
	}
	if got := price.For("us-east-1").EBSPerGiBMonth["gp3"]; !near(got, before) {
		t.Errorf("built-in gp3 changed from %v to %v", before, got)
	}
	if got := price.Default().EBSPerGiBMonth["gp3"]; !near(got, before) {
		t.Errorf("Default() gp3 = %v, want %v", got, before)
	}
}

func TestRatesFileRejectsMistakes(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"misspelt key", `{"elastic_ip_montly": 1}`, "elastic_ip_montly"},
		{"negative price", `{"nat_gateway_month": -5}`, "nat_gateway_month"},
		{"negative map entry", `{"ebs_per_gib_month": {"gp3": -1}}`, "ebs_per_gib_month.gp3"},
		{"null map", `{"ebs_per_gib_month": null}`, "ebs_per_gib_month"},
		{"wrong type", `{"elastic_ip_month": "cheap"}`, "elastic_ip_month"},
		{"not json", `elastic_ip_month: 1`, "--rates"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := price.Load(ratesFile(t, tc.body))
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// The built-in file must load through the same strict path, notes and all.
func TestBuiltInRatesFileLoadsAsCustom(t *testing.T) {
	if _, err := price.Load("../price/rates.json"); err != nil {
		t.Fatalf("a copy of rates.json is rejected: %v", err)
	}
}

// A bad file must fail before any AWS call, like every other bad flag.
func TestBadRatesFileFailsBeforeAnyAWSCall(t *testing.T) {
	root := cli.NewRootCommand("test", "test")
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"scan", "--no-config", "--region", "us-east-1",
		"--rates", ratesFile(t, `{"elastic_ip_montly": 1}`), "--profile", "no-such-profile-xyz"})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "elastic_ip_montly") {
		t.Fatalf("err = %v, want the rates error - did it reach AWS first?", err)
	}
}

// Custom prices reach the findings and are named in the report and its footer.
func TestCustomPricesReachTheReport(t *testing.T) {
	tbl, err := price.Load(ratesFile(t, `{"elastic_ip_month": 1}`))
	if err != nil {
		t.Fatal(err)
	}

	addr := fakeUnassociatedAddress()
	eng := &scan.Engine{
		Accounts: []awsapi.Factory{&fake.Factory{
			Clients: awsapi.Clients{EC2: addr, CW: &fake.CloudWatch{}, ELB: &fake.ELB{}, RDS: &fake.RDS{},
				EFS: &fake.EFS{}, KMS: &fake.KMS{}, ECR: &fake.ECR{}},
			Account: "123456789012",
			Base:    "us-east-1",
		}},
		Cfg:    detect.Defaults(),
		Clock:  func() time.Time { return now },
		Prices: tbl,
	}

	report, err := eng.Run(context.Background(), scan.Options{Only: []string{"eip-unassociated"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 || !near(report.Findings[0].MonthlyCost, 1) {
		t.Fatalf("findings = %+v, want one EIP at $1", report.Findings)
	}
	if !strings.HasPrefix(report.Prices, "custom:") {
		t.Errorf("report.Prices = %q", report.Prices)
	}

	var table, md bytes.Buffer
	if err := render.Table(&table, report, render.TableOptions{NoColor: true}); err != nil {
		t.Fatal(err)
	}
	if err := render.Markdown(&md, report, render.MarkdownOptions{}); err != nil {
		t.Fatal(err)
	}
	for name, out := range map[string]string{"table": table.String(), "markdown": md.String()} {
		if !strings.Contains(out, "my-prices.json") {
			t.Errorf("%s footer does not name the custom prices:\n%s", name, out)
		}
	}
}

func TestBuiltInPricesAreNamedInTheReport(t *testing.T) {
	eng := &scan.Engine{
		Accounts: []awsapi.Factory{&fake.Factory{
			Clients: awsapi.Clients{EC2: &fake.EC2{}, CW: &fake.CloudWatch{}, ELB: &fake.ELB{}, RDS: &fake.RDS{},
				EFS: &fake.EFS{}, KMS: &fake.KMS{}, ECR: &fake.ECR{}},
		}},
		Cfg:   detect.Defaults(),
		Clock: func() time.Time { return now },
	}
	report, err := eng.Run(context.Background(), scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Prices != "built-in "+price.Updated() {
		t.Errorf("report.Prices = %q", report.Prices)
	}
}
