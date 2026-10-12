package gooyai

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// maskedPassword stands in for a proxy password in what the dashboard sees.
const maskedPassword = "****"

// ErrInvalidProxy means a proxy URL isn't one Go's transport can dial.
var ErrInvalidProxy = errors.New(`proxy_url must be an http://, https://, socks5:// or socks5h:// URL with a host`)

// parseProxy validates raw as a proxy URL. Empty is valid: no override.
func parseProxy(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, nil //nolint:nilnil // nil = no proxy override
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return nil, ErrInvalidProxy
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
		return u, nil
	}
	return nil, ErrInvalidProxy
}

// redactProxy is raw with its password, if any, replaced by maskedPassword —
// what the dashboard is shown, since a proxy's credentials are as secret as
// the API key. An unparsable value is hidden entirely.
func redactProxy(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return maskedPassword
	}
	if _, has := u.User.Password(); !has {
		return u.String()
	}
	// url.UserPassword would escape the mask to %2A%2A%2A%2A, so it's spliced
	// in after the (escaped) user name instead.
	user := url.User(u.User.Username()).String()
	u.User = url.User(u.User.Username())
	return strings.Replace(u.String(), "://"+user+"@", "://"+user+":"+maskedPassword+"@", 1)
}

// restoreProxyPassword lets the dashboard send back the redacted proxy it was
// shown: a maskedPassword is swapped for the stored one, but only when the
// scheme, user and host still match, so a changed proxy never inherits
// credentials meant for another one.
func restoreProxyPassword(incoming, stored string) string {
	in, err := url.Parse(incoming)
	if err != nil || in.User == nil {
		return incoming
	}
	if pw, _ := in.User.Password(); pw != maskedPassword {
		return incoming
	}
	old, err := url.Parse(stored)
	if err != nil || old.User == nil {
		return incoming
	}
	oldPW, has := old.User.Password()
	if !has || in.Scheme != old.Scheme || in.Host != old.Host || in.User.Username() != old.User.Username() {
		return incoming
	}
	in.User = url.UserPassword(old.User.Username(), oldPW)
	return in.String()
}

// newProxiedHTTPClient routes requests through proxy, or the environment's
// proxy settings (HTTPS_PROXY etc.) when proxy is empty. proxy is already
// validated by parseProxy.
func newProxiedHTTPClient(proxy string) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if u, err := parseProxy(strings.TrimSpace(proxy)); err == nil && u != nil {
		transport.Proxy = http.ProxyURL(u)
	}
	return &http.Client{Timeout: clientTimeout, Transport: transport}
}
