package internal

// Status represents the status of a tunnel.
type Status int

//go:generate stringer -type=Status
const (
	// Undefined should never happen as a status or something is wrong.
	Undefined Status = iota
	// Close is the initial status of a tunnel, it means that nothing has
	// happened yet.
	Close
	// Opening is a "meta status" to display a transition from Close to Open.
	Opening
	// Open the tunnel is healthy.
	Open
	// Error means that the underlying tunnel process failed for some reason.
	// This will transition to Reopening.
	Error
	// Reopening follows an errored tunnel an will reattempt to open a tunnel.
	Reopening
	// PortBusy signals that a tunnel could not be opened due to a busy port.
	// This will transition to Reopening.
	PortBusy
	// Signal means that the underliyng process was signalled and terminated.
	// This will transition to Reopening.
	Signal
	// Cooper means that information about this tunnel's status is lost behind
	// the Event Horizon.
	// This will transition to Reopening.
	Cooper
)
