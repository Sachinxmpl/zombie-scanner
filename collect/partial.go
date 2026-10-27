package collect

import "fmt"

// Partial collects list of resources but some details are not collected
type Partial struct {
	Service   string
	Operation string
	Skipped   int
	Total     int
	What      string // "keys", "repositories"
	Hint      string
}

func (p *Partial) Error() string {
	msg := fmt.Sprintf("could not read %d of %d %s", p.Skipped, p.Total, p.What)
	if p.Hint != "" {
		msg += " (" + p.Hint + ")"
	}
	return msg
}
