package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"slices"
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
	"github.com/Sachinxmpl/zombie-scanner/render"
	"github.com/Sachinxmpl/zombie-scanner/scan"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func TestMultiAccountScan(t *testing.T) {
	volume := func(id string) *fake.EC2 {
		return &fake.EC2{
			DescribeVolumesFunc: func(context.Context, *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
				return &ec2.DescribeVolumesOutput{Volumes: []ec2types.Volume{{
					VolumeId: aws.String(id), State: ec2types.VolumeStateAvailable,
					VolumeType: ec2types.VolumeTypeGp3, Size: aws.Int32(100),
					CreateTime: aws.Time(daysAgo(45)),
				}}}, nil
			},
		}
	}
	account := func(id string, ec2api *fake.EC2) *fake.Factory {
		return &fake.Factory{
			Clients: awsapi.Clients{EC2: ec2api, CW: &fake.CloudWatch{}, ELB: &fake.ELB{}, RDS: &fake.RDS{}, EFS: &fake.EFS{}},
			Account: id,
			Base:    "us-east-1",
		}
	}

	eng := &scan.Engine{
		Accounts: []awsapi.Factory{
			account("111111111111", volume("vol-a")),
			account("222222222222", volume("vol-b")),
			// role missing, or account closed
			&fake.Factory{AccountErr: apiErr{code: "AccessDenied"}},
		},
		Cfg:   detect.Defaults(),
		Clock: func() time.Time { return now },
	}

	rep, err := eng.Run(context.Background(), scan.Options{})
	if err != nil {
		t.Fatalf("one bad account aborted the run: %v", err)
	}

	if !slices.Equal(rep.Accounts, []string{"111111111111", "222222222222"}) {
		t.Fatalf("accounts = %v", rep.Accounts)
	}
	if rep.AccountID != "111111111111" {
		t.Errorf("AccountID = %q, want the first account", rep.AccountID)
	}

	byID := map[string]string{}
	for _, f := range rep.Findings {
		byID[f.ResourceID] = f.AccountID
	}
	if byID["vol-a"] != "111111111111" || byID["vol-b"] != "222222222222" {
		t.Errorf("findings carry the wrong account: %v", byID)
	}

	if len(rep.Errors) != 1 || rep.Errors[0].Operation != "GetCallerIdentity" {
		t.Fatalf("errors = %+v", rep.Errors)
	}
	if rep.Errors[0].Kind != zombie.KindAccessDenied {
		t.Errorf("kind = %v, want access_denied", rep.Errors[0].Kind)
	}

	if !slices.Equal(rep.Regions, []string{"us-east-1"}) {
		t.Errorf("regions = %v, want one entry", rep.Regions)
	}
}

func TestAllAccountsUnreachableIsAnError(t *testing.T) {
	eng := &scan.Engine{
		Accounts: []awsapi.Factory{&fake.Factory{AccountErr: apiErr{code: "AccessDenied"}}},
		Cfg:      detect.Defaults(),
		Clock:    func() time.Time { return now },
	}
	if _, err := eng.Run(context.Background(), scan.Options{}); err == nil {
		t.Fatal("no account resolved, but Run succeeded")
	}
}

func TestNoAccountsConfiguredIsAnError(t *testing.T) {
	eng := &scan.Engine{Cfg: detect.Defaults(), Clock: func() time.Time { return now }}
	if _, err := eng.Run(context.Background(), scan.Options{}); err == nil {
		t.Fatal("empty Accounts, but Run succeeded")
	}
}

func TestErrorRenderingWithoutARegion(t *testing.T) {
	errs := []zombie.ScanError{
		{Service: "ec2", Operation: "DescribeVolumes", Region: "us-east-1",
			Kind: zombie.KindAccessDenied, Message: "denied"},
	}
	// every kind, because each has its own branch in the renderer
	for _, kind := range []zombie.ErrorKind{
		zombie.KindAccessDenied, zombie.KindThrottled,
		zombie.KindUnsupported, zombie.KindOther,
	} {
		errs = append(errs, zombie.ScanError{
			Service: "sts", Operation: "GetCallerIdentity" + string(kind),
			Kind: kind, Message: "cannot assume role",
		})
	}
	rep := zombie.Report{Errors: errs}

	for _, verbose := range []bool{false, true} {
		var buf bytes.Buffer
		render.Errors(&buf, rep, verbose)
		out := buf.String()

		if strings.Contains(out, "()") {
			t.Errorf("verbose=%v: empty region rendered as (): %q", verbose, out)
		}
		if strings.Contains(out, "in :") {
			t.Errorf("verbose=%v: empty region rendered as 'in :': %q", verbose, out)
		}
		if !strings.Contains(out, "us-east-1") {
			t.Errorf("verbose=%v: a real region was dropped: %q", verbose, out)
		}
	}
}

func TestSortIsTotalAcrossAccounts(t *testing.T) {
	build := func() []zombie.Finding {
		return []zombie.Finding{
			{ResourceID: "database-1", AccountID: "222222222222", Region: "us-east-1",
				Confidence: zombie.High, MonthlyCost: 2.48},
			{ResourceID: "database-1", AccountID: "111111111111", Region: "us-east-1",
				Confidence: zombie.High, MonthlyCost: 2.48},
		}
	}

	// many runs, because an unstable sort can come out right by luck
	want := []string{"111111111111", "222222222222"}
	for i := range 50 {
		got := []string{}
		for _, f := range render.Sort(build()) {
			got = append(got, f.AccountID)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("run %d: got %v, want %v", i, got, want)
		}
	}
}

func TestIAMPolicyEncodesBothResourceShapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"scanned account", []string{"iam-policy"},
			[]string{`"Resource": "*"`, "ec2:DescribeVolumes", "rds:DescribeDBInstances"}},
		{"assume role", []string{"iam-policy", "--assume-role", "--accounts", "111111111111"},
			[]string{"sts:AssumeRole", "arn:aws:iam::111111111111:role/ZombieScannerReadOnly"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			root := cli.NewRootCommand("test", "test")
			root.SetOut(&out)
			root.SetErr(io.Discard)
			root.SetArgs(tc.args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}

			// check the encoded JSON: a dropped field is only visible there
			var policy map[string]any
			if err := json.Unmarshal(out.Bytes(), &policy); err != nil {
				t.Fatalf("not valid JSON: %v\n%s", err, out.String())
			}
			stmts := policy["Statement"].([]any)
			if _, ok := stmts[0].(map[string]any)["Resource"]; !ok {
				t.Fatalf("Resource dropped from output: %s", out.String())
			}
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in:\n%s", want, out.String())
				}
			}
		})
	}
}
