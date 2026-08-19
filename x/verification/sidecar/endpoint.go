package sidecar

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

var (
	// ErrInvalidGRPCAddress reports an unsafe or malformed sidecar endpoint.
	ErrInvalidGRPCAddress = errors.New("invalid verification sidecar gRPC address")
	// ErrInvalidTransport reports an unsupported sidecar transport policy.
	ErrInvalidTransport = errors.New("invalid verification sidecar transport mode")
)

// TransportMode controls which plaintext endpoint classes a client may use.
// The modes are intentionally narrow because verifier RPC traffic is not
// authenticated or encrypted in this phase.
type TransportMode string

const (
	// TransportModeLoopback permits only literal loopback IP endpoints.
	TransportModeLoopback TransportMode = "loopback"
	// TransportModeTrustedNetwork permits private IP and DNS service endpoints.
	TransportModeTrustedNetwork TransportMode = "trusted-network"

	// EnabledConfigKey identifies the sidecar enable switch in app.toml.
	EnabledConfigKey = "verification.enabled"
	// GRPCAddressConfigKey identifies the sidecar gRPC endpoint in app.toml.
	GRPCAddressConfigKey = "verification.grpc_address"
	// TransportModeConfigKey identifies the sidecar endpoint policy in app.toml.
	TransportModeConfigKey = "verification.grpc_transport_mode"
	// RequestTimeoutConfigKey identifies the result RPC timeout in app.toml.
	RequestTimeoutConfigKey = "verification.request_timeout"
)

// ParseTransportMode parses an exact app.toml transport mode value.
func ParseTransportMode(value string) (TransportMode, error) {
	if value != strings.TrimSpace(value) {
		return "", fmt.Errorf("%w: surrounding whitespace is not allowed", ErrInvalidTransport)
	}
	mode := TransportMode(value)
	if mode != TransportModeLoopback && mode != TransportModeTrustedNetwork {
		return "", fmt.Errorf("%w: unsupported mode %q", ErrInvalidTransport, value)
	}
	return mode, nil
}

func validateGRPCAddress(address string, mode TransportMode) error {
	if mode != TransportModeLoopback && mode != TransportModeTrustedNetwork {
		return ErrInvalidTransport
	}
	if address != strings.TrimSpace(address) {
		return fmt.Errorf("%w: surrounding whitespace is not allowed", ErrInvalidGRPCAddress)
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: expected host:port: %v", ErrInvalidGRPCAddress, err)
	}
	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort == 0 {
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrInvalidGRPCAddress)
	}

	// Literal IPs can be classified without DNS, keeping startup validation
	// deterministic and preventing obvious public plaintext endpoints before
	// any connection attempt is made.
	ip, err := netip.ParseAddr(host)
	if err == nil {
		if ip.Zone() != "" || ip.IsUnspecified() {
			return fmt.Errorf("%w: invalid client host %q", ErrInvalidGRPCAddress, host)
		}
		if mode == TransportModeLoopback && !ip.IsLoopback() {
			return fmt.Errorf("%w: host %q is not loopback", ErrInvalidGRPCAddress, host)
		}
		if mode == TransportModeTrustedNetwork && !ip.IsLoopback() && !ip.IsPrivate() {
			return fmt.Errorf("%w: host %q is neither loopback nor private", ErrInvalidGRPCAddress, host)
		}
		return nil
	}

	// DNS names are accepted only in trusted-network mode for container and
	// cluster service discovery. A DNS name cannot be proven private locally, so
	// operators must enforce the trust boundary with network policy.
	if mode != TransportModeTrustedNetwork {
		return fmt.Errorf("%w: loopback mode requires a literal IP", ErrInvalidGRPCAddress)
	}
	if looksLikeIPv4Literal(host) || !isValidDNSName(host) {
		return fmt.Errorf("%w: invalid DNS service name %q", ErrInvalidGRPCAddress, host)
	}
	return nil
}

func looksLikeIPv4Literal(host string) bool {
	if !strings.Contains(host, ".") {
		return false
	}
	for _, char := range host {
		if (char < '0' || char > '9') && char != '.' {
			return false
		}
	}
	return true
}

func isValidDNSName(host string) bool {
	if host == "" || len(host) > 253 || strings.HasSuffix(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') &&
				(char < 'A' || char > 'Z') &&
				(char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}
