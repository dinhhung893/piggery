//go:build windows

package server

import (
	"errors"
	"net"
)

// peerPID is the pid of the process at the other end of a unix socket. Windows has no
// SO_PEERCRED/LOCAL_PEERPID equivalent reachable from Go's net API, so peer-based checks
// (byHost auth, the worker-bound identify gate) deny on windows; token auth is unaffected.
func peerPID(nc net.Conn) (int, error) {
	return 0, errors.New("peer pid is not supported on windows")
}
