package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"net"
	"strings"
)

// A brand-new Cairn has no user, and creating that user is necessarily
// unauthenticated -- there is nobody to authenticate as yet. On a laptop that
// is fine. On a server with a domain pointed at it, it means whoever reaches
// the URL first owns the tracker, and the real owner is met with "this Cairn
// has already been set up".
//
// So first-launch setup asks for a code printed to the server log, and only
// when the server is reachable from somewhere other than this machine. Running
// `./cairn` on a laptop binds loopback and asks for nothing; a container
// binding 0.0.0.0 behind a reverse proxy asks for the code. The friction lands
// where the risk is.
//
// The code lives in memory for the life of the process. A restart mints a new
// one, which is the behaviour you want: an old code seen over someone's
// shoulder stops working.

// setupCodeAlphabet omits characters that are misread when copied by hand.
const setupCodeAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// NewSetupCode returns a grouped code such as "kf9d-2mqx-7rvt-a4bn".
func NewSetupCode() string {
	const groups, size = 4, 4
	buf := make([]byte, groups*size)
	rand.Read(buf)

	var b strings.Builder
	for i, v := range buf {
		if i > 0 && i%size == 0 {
			b.WriteByte('-')
		}
		b.WriteByte(setupCodeAlphabet[int(v)%len(setupCodeAlphabet)])
	}
	return b.String()
}

// ListenerIsLocalOnly reports whether an address can only be reached from this
// machine. An empty host, as in ":7777", means every interface.
func ListenerIsLocalOnly(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// matchesSetupCode compares in constant time, and treats an unset code as
// "no code required".
func (s *Server) matchesSetupCode(given string) bool {
	if s.setupCode == "" {
		return true
	}
	want := strings.ToLower(strings.TrimSpace(s.setupCode))
	got := strings.ToLower(strings.TrimSpace(given))
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}
