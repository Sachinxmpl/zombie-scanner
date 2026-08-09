package zombie

import "fmt"

type ErrorKind string

const (
	KindAccessDenied ErrorKind = "access_denied"
	KindThrottled    ErrorKind = "throttled"
	KindUnsupported  ErrorKind = "unsupported"
	KindOther        ErrorKind = "other"
)

type ScanError struct {
	Region    string    `json:"region"`
	Service   string    `json:"service"`   // ec2
	Operation string    `json:"operation"` // DescribeVolumes
	Kind      ErrorKind `json:"kind"`
	Message   string    `json:"message"`
}

func (e ScanError) Error() string {
	return fmt.Sprintf("%s:%s in %s: %s", e.Service, e.Operation, e.Region, e.Message)
}
