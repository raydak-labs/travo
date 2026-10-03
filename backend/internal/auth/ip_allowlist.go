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
// outside ALLOWED_ADMIN_CIDRS.
//
// It is a subset of PublicPaths, not a second independent list: the two used to
// drift, and the drift is invisible at runtime because neither list is checked
// against the route table.
//
// /api/v1/ws is deliberately NOT exempt. It authenticates itself with a session
// token, but it is also a live administrative channel, so the IP allowlist --
// "only administer this router from my laptop" -- must still apply to it. The
// remaining public paths are bootstrap or read-only: the health probe, the
// machine-readable API contract for automation, login, and the pre-login clock
// recovery that an operator needs when the device's clock is wrong.
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
