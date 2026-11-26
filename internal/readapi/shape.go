package readapi

import (
	"encoding/hex"
	"net"
	"net/http"
	"strings"
)

// tagBytes is the length of a shape tag: enough to tell addresses apart in one
// test, far too short to look one up.
const tagBytes = 4

const headerXFF = "X-Forwarded-For"

// shapeHeaders are the headers a proxy may put a client address in.
func shapeHeaders() []string {
	return []string{
		headerXFF, "Forwarded", "X-Real-IP", "True-Client-IP", "Fastly-Client-IP",
		"X-Appengine-User-IP", "X-Client-IP", "Via",
	}
}

// logShape is the RATE_LIMIT_DEBUG diagnostic (M1-10 cloud check): it logs
// which address headers reached read-api and how X-Forwarded-For is built,
// so the operator can choose CLIENT_IP_HEADER. It never logs a value: each
// address appears only as a short tag, the HMAC of the address and the day
// under the limiter's in-memory key, so the same address gives the same tag
// for one process and one day, and no tag can be reversed.
func (l *limiter) logShape(r *http.Request) {
	if l.log == nil {
		return
	}
	now := l.now()
	tag := func(v string) string {
		k := l.clientKey(strings.TrimSpace(v), now)
		return hex.EncodeToString(k[:tagBytes])
	}
	present := []string{}
	others := map[string]string{}
	for _, h := range shapeHeaders() {
		v := r.Header.Get(h)
		if v == "" {
			continue
		}
		present = append(present, h)
		if h != headerXFF {
			others[h] = tag(v)
		}
	}
	xff := []string{}
	if v := r.Header.Get(headerXFF); v != "" {
		for p := range strings.SplitSeq(v, ",") {
			xff = append(xff, tag(p))
		}
	}
	socket, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		socket = r.RemoteAddr
	}
	l.log.InfoContext(r.Context(), "client address shape",
		"class", classify(r.URL.Path), "present", present,
		"xff_count", len(xff), "xff_tags", xff, "other_tags", others,
		"socket_tag", tag(socket), "keyed_tag", tag(l.clientAddr(r)))
}
