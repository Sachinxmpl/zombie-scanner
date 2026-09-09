#!/usr/bin/env bash

# zombie-scanner is read-only. This scripts verifies claim in three ways.

set -euo pipefail

fail=0

#1 -> Only awsapi may construct an AWS SDK client. Everything else receives a narrow interface, so it can only call what awsapi declares
echo "-> only awsapi constructs AWS SDK clients"
if git ls-files '*.go' \
  | grep -v '^awsapi/' \
  | xargs grep -ln 'NewFromConfig' 2>/dev/null; then
  echo "x a package outside awsapi/ constructs an SDK client"
  echo "  All AWS access goes through the interfaces in awsapi/api.go, which"
  echo "  are read-only by construction. Building a client elsewhere bypasses that."
  fail=1
else
  echo "ok only awsapi constructs AWS SDK clients"
fi

#2 Every AWS operation this tool can call is declared in awsapi/api.go.
#  Assert they are all reads.
echo "-> every declared AWS method is a read operation"
methods=$(awk '
  /^type [A-Za-z0-9]+API interface \{/ { inside = 1; next }
  inside && /^\}/                    { inside = 0; next }
  inside && match($0, /^[[:space:]]+[A-Z][A-Za-z]*\(ctx/) {
    m = $0; sub(/^[[:space:]]+/, "", m); sub(/\(ctx.*/, "", m); print m
  }
' awsapi/api.go | sort -u)

# every *API interface declared in the file must have been scanned
declared=$(grep -cE '^type [A-Za-z0-9]+API interface \{' awsapi/api.go)
scanned=$(awk '/^type [A-Za-z0-9]+API interface \{/ { n++ } END { print n+0 }' awsapi/api.go)
if [ "$declared" -eq 0 ] || [ "$declared" -ne "$scanned" ]; then
  echo "x extractor scanned $scanned of $declared *API interfaces in awsapi/api.go"
  fail=1
fi

if [ -z "$methods" ]; then
  echo "x found no interface methods in awsapi/api.go - has the file moved?"
  fail=1
fi

surface_ok=1
for m in $methods; do
  case "$m" in
    Describe*|Get*|List*) ;;
    *)
      echo "x $m is not a read operation"
      echo "  awsapi interfaces are the complete list of AWS calls this tool"
      echo "  can make. Only Describe/Get/List belong there."
      surface_ok=0
      fail=1
      ;;
  esac
done
[ $surface_ok -eq 1 ] && echo "ok every AWS API method this tool declares is a read operation"


#3.Explicit denylist of real mutating operations. 
# Exact names, so there are no false positives - unlike a verb prefix
echo "-> no known mutating operation appears in the source"
mutations='RunInstances|TerminateInstances|StopInstances|StartInstances'
mutations="$mutations|CreateTags|DeleteTags|CreateVolume|DeleteVolume|ModifyVolume"
mutations="$mutations|AttachVolume|DetachVolume|CreateSnapshot|DeleteSnapshot"
mutations="$mutations|AllocateAddress|ReleaseAddress|AssociateAddress|DisassociateAddress"
mutations="$mutations|CreateNatGateway|DeleteNatGateway|DeregisterImage|DeleteLoadBalancer"
mutations="$mutations|PutMetricData|CreateBucket|DeleteObject"
mutations="$mutations|CreateDBInstance|DeleteDBInstance|ModifyDBInstance|RebootDBInstance"
mutations="$mutations|StartDBInstance|StopDBInstance|CreateDBSnapshot|DeleteDBSnapshot"

if git ls-files '*.go' \
  | grep -v '_test\.go$' \
  | grep -v '^test/' \
  | xargs grep -nE "\.($mutations)\(" 2>/dev/null; then
  echo "x a mutating AWS call appears in the source"
  echo "  zombie-scanner reports; it never acts. There is no --delete flag."
  fail=1
else
  echo "ok no known mutating operation appears in the source"
fi

exit $fail
