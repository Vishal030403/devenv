package portalloc

import (
	"fmt"
	"net"
)

const (
	preferredAppPort = 4173
	rangeStart       = 19000
	rangeEnd         = 20000
	// JenkinsReservedLocalPort is the ONLY localhost TCP port Jenkins may bind for kubectl port-forward UI.
	// Application exposure must never allocate or advertise this host port.
	JenkinsReservedLocalPort = 8080
)

// AllocateApplicationLocalPort picks a free localhost TCP port for kubectl port-forward to the workload Service.
func AllocateApplicationLocalPort() (int, error) {
	if p := tryAlloc(preferredAppPort); p >= 0 {
		return finalize(p)
	}
	for p := rangeStart; p <= rangeEnd; p++ {
		if p == JenkinsReservedLocalPort {
			continue
		}
		if q := tryAlloc(p); q >= 0 {
			return finalize(q)
		}
	}
	return 0, fmt.Errorf("no free localhost port outside Jenkins reservation :%d in range prefer %d and %d-%d",
		JenkinsReservedLocalPort, preferredAppPort, rangeStart, rangeEnd)
}

func tryAlloc(port int) int {
	if port == JenkinsReservedLocalPort {
		return -1
	}
	if !isFree(port) {
		return -1
	}
	return port
}

func finalize(port int) (int, error) {
	if port == JenkinsReservedLocalPort || port <= 0 {
		return 0, fmt.Errorf("internal invariant failed: workload listener must not bind localhost:%d", JenkinsReservedLocalPort)
	}
	return port, nil
}

func isFree(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	if port == JenkinsReservedLocalPort {
		return false
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
