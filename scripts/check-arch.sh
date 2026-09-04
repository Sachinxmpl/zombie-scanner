#!/usr/bin/env bash

# Pure packages like detect, price, filter ....-> check if they have AWS SDK dependency?

set -euo pipefail

fail=0
for pkg in zombie detect price filter render; do
  if go list -deps "./$pkg" | grep -q 'aws-sdk-go-v2'; then
    echo "x $pkg depends on the AWS SDK"
    echo
    echo "  $pkg must stay pure: no network, no clock, no SDK."
    echo "  Collect the data in collect/ and pass it in as a zombie.Inventory."
    echo
    echo "  Offending import chain:"
    go list -deps "./$pkg" | grep 'aws-sdk-go-v2' | head -3 | sed 's/^/    /'
    echo
    fail=1
  fi
done

[ $fail -eq 0 ] && echo "ok detect, price, filter, render, zombie have no AWS SDK dependency"
exit $fail
