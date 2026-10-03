//go:build windows

package server

// peerGateSupported reports whether the platform can identify the socket peer's pid.
// Windows has no SO_PEERCRED equivalent reachable from Go's net API, so the peer gate
// (peerWorkerOK, peerHostOK) cannot be enforced. Token auth remains the security
// boundary; on a single-user machine the profile-dir ACL supplements it. Documented
// trade-off: a harness nested in a worker can speak for it (the nesting guard is off).
func peerGateSupported() bool { return false }
