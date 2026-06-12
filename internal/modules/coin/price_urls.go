package coin

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func validateEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("coin: invalid API URL %q", raw)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && isLocalHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("coin: API URL must be https: %s", raw)
}

func isLocalHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
