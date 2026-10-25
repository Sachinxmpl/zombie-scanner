package test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
)

// records whether a request got past the guard (doesnot actually sends it)
type fakeNetwork struct{ sent bool }

func (n *fakeNetwork) Do(*http.Request) (*http.Response, error) {
	n.sent = true
	return nil, errors.New("fake network")
}

var guarded = []func(*middleware.Stack) error{awsapi.ReadOnly}
var fakeCreds = credentials.NewStaticCredentialsProvider("AKID", "SECRET", "")

func guardedEC2(n *fakeNetwork) *ec2.Client {
	return ec2.New(ec2.Options{Region: "us-east-1", Credentials: fakeCreds, HTTPClient: n,
		APIOptions: guarded, RetryMaxAttempts: 1})
}

func guardedSTS(n *fakeNetwork) *sts.Client {
	return sts.New(sts.Options{Region: "us-east-1", Credentials: fakeCreds, HTTPClient: n,
		APIOptions: guarded, RetryMaxAttempts: 1})
}

func TestReadOnlyGuardRefusesWritesBeforeTheNetwork(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		call func(n *fakeNetwork) error
		want string
	}{
		{"ec2 write", func(n *fakeNetwork) error {
			_, err := guardedEC2(n).TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: []string{"i-1"}})
			return err
		}, "ec2:TerminateInstances"},
		{"sts write other than AssumeRole", func(n *fakeNetwork) error {
			_, err := guardedSTS(n).AssumeRoleWithWebIdentity(ctx, &sts.AssumeRoleWithWebIdentityInput{
				RoleArn: aws.String("arn:aws:iam::111111111111:role/x"), RoleSessionName: aws.String("x"),
				WebIdentityToken: aws.String("x")})
			return err
		}, "sts:AssumeRoleWithWebIdentity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := &fakeNetwork{}
			err := tc.call(n)
			if !errors.Is(err, awsapi.ErrNotReadOnly) {
				t.Fatalf("err = %v, want ErrNotReadOnly", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not name %s", err, tc.want)
			}
			if n.sent {
				t.Error("the request reached the network")
			}
		})
	}
}

func TestReadOnlyGuardLetsReadsAndAssumeRoleThrough(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		call func(n *fakeNetwork) error
	}{
		{"ec2 read", func(n *fakeNetwork) error {
			_, err := guardedEC2(n).DescribeVolumes(ctx, &ec2.DescribeVolumesInput{})
			return err
		}},
		{"kms List read", func(n *fakeNetwork) error {
			_, err := kms.New(kms.Options{Region: "us-east-1", Credentials: fakeCreds, HTTPClient: n,
				APIOptions: guarded, RetryMaxAttempts: 1}).ListKeys(ctx, &kms.ListKeysInput{})
			return err
		}},
		{"sts Get read", func(n *fakeNetwork) error {
			_, err := guardedSTS(n).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
			return err
		}},
		{"sts AssumeRole for --accounts", func(n *fakeNetwork) error {
			_, err := guardedSTS(n).AssumeRole(ctx, &sts.AssumeRoleInput{
				RoleArn: aws.String("arn:aws:iam::111111111111:role/x"), RoleSessionName: aws.String("x")})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := &fakeNetwork{}
			err := tc.call(n)
			if errors.Is(err, awsapi.ErrNotReadOnly) {
				t.Fatalf("refused a call that must be allowed: %v", err)
			}
			if !n.sent {
				t.Errorf("never reached the network: %v", err)
			}
		})
	}
}
