package zombie

import "time"

// Inventory contains resources + clock for one region

type Inventory struct {
	Region    string
	AccountID string

	Now time.Time

	Volumes       []Volume
	Addresses     []Address
	Snapshots     []Snapshot
	Images        []Image
	Instances     []Instance
	NATGateways   []NATGateway
	LoadBalancers []LoadBalancer
}

type Volume struct {
	ID         string
	State      string // available -> unattached
	VolumeType string // gp3, gp2, io1, io2
	SizeGiB    int32
	CreatedAt  time.Time
	AttachedTo string // instanceid
	Tags       map[string]string
}

type Address struct {
	AllocationID  string
	PublicIP      string
	AssociationID string // empty -> unassociated
	Tags          map[string]string
}

type Snapshot struct {
	ID          string
	VolumeID    string
	SizeGiB     int32
	StartedAt   time.Time
	Description string
	Tags        map[string]string
}

type Image struct {
	ID          string
	Name        string
	SnapshotIDs []string
	CreatedAt   time.Time
}

type Instance struct {
	ID                    string
	Type                  string
	State                 string
	StateTransitionReason string
	VolumeIDs             []string
	LaunchedAt            time.Time
	Tags                  map[string]string
}

type NATGateway struct {
	ID        string
	VPCID     string
	State     string
	CreatedAt time.Time
	Tags      map[string]string
}

type LoadBalancer struct {
	ARN          string
	Name         string
	Type         string
	MetricSuffix string
	CreatedAt    time.Time
	Tags         map[string]string
}
