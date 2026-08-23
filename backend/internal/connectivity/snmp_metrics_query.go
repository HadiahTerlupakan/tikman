package connectivity

// ONTMetrics represents collected metrics from an ONT.
// RxPower/TxPower are pointers because ZTE returns a sentinel value when there
// is no optical signal - a nil pointer means "no reading", which must not be
// confused with a genuine 0.00 dBm measurement.
type ONTMetrics struct {
	RxPower         *float64 // in dBm, nil when no signal
	TxPower         *float64 // in dBm, nil when no signal
	Temperature     float64  // in Celsius
	Voltage         float64  // in Volts
	TxBiasCurrent   float64  // in mA
	Distance        int      // in meters
	RxBytes         uint64
	TxBytes         uint64
	RxPackets       uint64
	TxPackets       uint64
	RxErrors        uint64
	TxErrors        uint64
	SerialNumber    string // ONU serial number
	SoftwareVersion string // Equipment ID / firmware version (e.g. F660V9)
}
