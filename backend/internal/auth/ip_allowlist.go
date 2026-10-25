package auth

import (
	"fmt"
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
)

func ParseCIDRList(raw string) ([]*net.IPNet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var out []*net.IPNet
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if !strings.Contains(part, "/") {
			ip := net.ParseIP(part)
			if ip == nil {
				return nil, fmt.Errorf("invalid IP %q", part)
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			part = fmt.Sprintf("%s/%d", ip.String(), bits)
		}

		_, n, err := net.ParseCIDR(part)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", part, err)
		}
		out = append(out, n)
	}

	return out, nil
}

func isLoopbackIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func clientIP(c fiber.Ctx) net.IP {
	return net.ParseIP(strings.TrimSpace(c.IP()))
}

func IPAllowed(nets []*net.IPNet, ip net.IP) bool {
	if len(nets) == 0 {
		return true
	}
	if isLoopbackIP(ip) {
		return true
	}
	for _, n := range nets {
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}

// shouldBypassIPAllowlist reports whether a path is reachable from a client IP
// outside ALLOWED_ADMIN_CIDRS. It is a subset of PublicPaths, derived from it so
// the two cannot drift.
//
// /api/v1/ws is NOT exempt: it is a live administrative channel, so "only
// administer this router from my laptop" applies to it. The exempt paths are
// bootstrap or read-only: health, the API contract, login and the pre-login
// clock recovery.
func shouldBypassIPAllowlist(path string) bool {
	if path == "/api/v1/ws" {
		return false
	}
	return PublicPaths[path]
}

func IPAllowlistMiddleware(nets []*net.IPNet) fiber.Handler {
	return func(c fiber.Ctx) error {
		if len(nets) == 0 {
			return c.Next()
		}

		if shouldBypassIPAllowlist(c.Path()) {
			return c.Next()
		}

		ip := clientIP(c)
		if !IPAllowed(nets, ip) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "forbidden: client IP not allowed",
			})
		}

		return c.Next()
	}
}
