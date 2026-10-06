package filter

import (
	"sync/atomic"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

type ManagedSet interface {
	Has(string) bool
}

type Unmanaged struct {
	Managed ManagedSet
	dropped atomic.Int64
}

func (*Unmanaged) Name() string {
	return "--tf-state"
}

func (u *Unmanaged) Keep(f zombie.Finding) bool {
	if u.Managed.Has(f.ResourceID) || u.Managed.Has(f.ResourceARN) {
		u.dropped.Add(1)
		return false
	}
	return true
}

func (u *Unmanaged) Dropped() int64 {
	return u.dropped.Load()
}
