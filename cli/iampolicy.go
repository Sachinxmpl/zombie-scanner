package cli

import (
	"encoding/json"
	"slices"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/spf13/cobra"
)

type iamStatement struct {
	Effect string   `json:"Effect"`
	Action []string `json:"Action"`

	Resource any `json:"Resource"`
}

type iamPolicy struct {
	Version   string         `json:"Version"`
	Statement []iamStatement `json:"Statement"`
}

func newIAMPolicyCommand(o *options) *cobra.Command {
	var assumeRole bool

	cmd := &cobra.Command{
		Use:   "iam-policy",
		Short: "Print the minimal IAM policy this tool needs",
		Long: `Print the minimal IAM policy this tool needs.
The policy is generated from each detector's declared permissions, so it can 
never drift from what the code actually calls. Use --only or --skip to get a
policy for a subset of detectors.

Resource is "*" because the EC2 Describe actions do not support resource-level permissions.

This policy goes on the role in each scanned account. With --accounts, your own
login also needs sts:AssumeRole on those roles: print it with --assume-role.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateDetectors(o.Only, o.Skip); err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)

			if assumeRole {
				return enc.Encode(assumePolicy(o.RoleName, o.Accounts))
			}
			return enc.Encode(buildPolicy(o.Only, o.Skip))
		},
	}

	cmd.Flags().BoolVar(&assumeRole, "assume-role", false,
		"print the sts:AssumeRole policy your own login needs instead")
	return cmd
}

// lets the caller assume the role in each target account, or in any account when none are named
func assumePolicy(roleName string, accounts []string) iamPolicy {
	resources := []string{}
	for _, id := range accounts {
		resources = append(resources, "arn:aws:iam::"+id+":role/"+roleName)
	}
	if len(resources) == 0 {
		resources = []string{"arn:aws:iam::*:role/" + roleName}
	}
	slices.Sort(resources)

	return iamPolicy{
		Version: "2012-10-17",
		Statement: []iamStatement{{
			Effect:   "Allow",
			Action:   []string{"sts:AssumeRole"},
			Resource: resources,
		}},
	}
}

// Union of factory's own calls and every selected detector's Needs()
func buildPolicy(only, skip []string) iamPolicy {
	seen := map[string]bool{}
	for _, a := range awsapi.RequiredActions {
		seen[a] = true
	}

	onlySet, skipSet := toSet(only), toSet(skip)
	for _, d := range detect.All() {
		name := d.Name()
		if len(onlySet) > 0 && !onlySet[name] {
			continue
		}
		if skipSet[name] {
			continue
		}
		for _, a := range d.Needs() {
			seen[a] = true
		}
	}

	actions := make([]string, 0, len(seen))
	for a := range seen {
		actions = append(actions, a)
	}
	slices.Sort(actions)

	return iamPolicy{
		Version: "2012-10-17",
		Statement: []iamStatement{
			{
				Effect:   "Allow",
				Action:   actions,
				Resource: "*",
			},
		},
	}
}

func toSet(xs []string) map[string]bool {
	if len(xs) == 0 {
		return nil
	}
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}
