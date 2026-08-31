package render

import (
	"encoding/json"
	"io"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// JSON writes the report as indented JSON.
// The keys are a public API the moment someone pipes this into jq: snake_case,
// never renamed within a major version, and findings/errors are never null.
func JSON(w io.Writer, r zombie.Report) error {
	r.Normalize()

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	return enc.Encode(r)
}
