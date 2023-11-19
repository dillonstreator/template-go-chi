package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

var trustedIPs = []string{
	"192.168.0.0/16",
	"172.16.0.0/12",
	"10.0.0.0/8",
	"127.0.0.1/8",
	"fd00::/8",
	"::1",
}
var parsedTrustedIPs = parseIPs(trustedIPs)

var proxyIPHeaders = []string{
	"X-Envoy-External-Address",
	"X-Forwarded-For",
	"X-Real-IP",
	"True-Client-IP",
}

var schemeHeaders = []string{
	"X-Forwarded-Proto",
	"X-Forwarded-Scheme",
}

var xForwardedHost = "X-Forwarded-Host"

func trustProxy(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			trusted, err := isTrustedIP(r.RemoteAddr, parsedTrustedIPs)
			if err != nil {
				logger.Error(err.Error(), slog.String("ip", r.RemoteAddr))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if !trusted {
				next.ServeHTTP(w, r)
				return
			}

			if realIP := getRealIP(r.Header); realIP != "" {
				r.RemoteAddr = realIP
			}

			if host := r.Header.Get(xForwardedHost); host != "" {
				r.Host = host
			}

			if scheme := getScheme(r.Header); scheme != "" {
				r.URL.Scheme = scheme
			}

			next.ServeHTTP(w, r)
		})
	}
}

func parseIPs(ips []string) []netip.Prefix {
	var parsedIPs []netip.Prefix

	for _, ipStr := range ips {
		if strings.Contains(ipStr, "/") {
			ipNet, err := netip.ParsePrefix(ipStr)
			if err != nil {
				panic(fmt.Sprintf("parsing CIDR expression: %s", err.Error()))
			}

			parsedIPs = append(parsedIPs, ipNet)
		} else {
			ipAddr, err := netip.ParseAddr(ipStr)
			if err != nil {
				panic(fmt.Sprintf("invalid IP address: '%s': %s", ipStr, err.Error()))
			}

			parsedIPs = append(parsedIPs, netip.PrefixFrom(ipAddr, ipAddr.BitLen()))
		}
	}

	return parsedIPs
}

func isTrustedIP(remoteAddr string, trustedIPs []netip.Prefix) (bool, error) {
	ipStr, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		ipStr = remoteAddr
	}

	ipAddr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return false, err
	}

	for _, ipRange := range trustedIPs {
		if ipRange.Contains(ipAddr) {
			return true, nil
		}
	}

	return false, nil
}

func getRealIP(headers http.Header) string {
	var addr string

	for _, proxyHeader := range proxyIPHeaders {
		if value := headers.Get(proxyHeader); value != "" {
			addr = strings.SplitN(value, ",", 2)[0]
			break
		}
	}

	return addr
}

func getScheme(headers http.Header) string {
	var scheme string

	for _, schemaHeader := range schemeHeaders {
		if value := headers.Get(schemaHeader); value != "" {
			scheme = strings.ToLower(scheme)
			break
		}
	}

	return scheme
}
