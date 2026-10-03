//go:build !windows

package server

// peerGateSupported reports whether the platform can identify the socket peer's pid.
// On unix, SO_PEERCRED/LOCAL_PEERPID provides this; the peer gate is enforced.
func peerGateSupported() bool { return true }
