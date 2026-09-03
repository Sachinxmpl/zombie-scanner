package cli

import "errors"

var ErrSpendAboveThreshold = errors.New("zombie spend above --fail-if-above threshold")
