package status

// Status represents the normalized health state of a monitor.
// In ascending severity: Unknown < Normal < Degraded < Outage.
type Status int

const (
	// Unknown is the lowest severity. Fetch failures result in Unknown.
	// Color: #9E9E9E
	Unknown Status = iota
	// Normal means the service is fully operational.
	// Color: #69B72A
	Normal
	// Degraded means the service is experiencing partial outage, performance degradation.
	// Color: #F0E442
	Degraded
	// Outage means the service is experiencing a major outage or under maintenance.
	// Color: #D50000
	Outage
)

// String returns the string representation of the Status.
func (s Status) String() string {
	switch s {
	case Unknown:
		return "Unknown"
	case Normal:
		return "Normal"
	case Degraded:
		return "Degraded"
	case Outage:
		return "Outage"
	default:
		return "Unknown"
	}
}
