// Package gooyai exposes a clinic's Gooyai (AI phone receptionist) data to
// the dashboard. The Partner API client itself lives in pkg/gooyai; this
// package owns each clinic's connection (its key, encrypted here and stored in
// the clinic's user settings, see usersRepository) and how results map to our
// proto.
package gooyai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/salahfarzin/meet/pkg/gooyai"
)

var (
	// ErrNotConnected means the clinic has no Gooyai connection.
	ErrNotConnected = errors.New("clinic is not connected to the phone assistant")
	// ErrKeyRejected means Gooyai refused the key while connecting.
	ErrKeyRejected = errors.New("the phone assistant provider rejected this API key")
	// ErrInvalidConnection covers a blank key with nothing stored to keep, or
	// an unknown environment.
	ErrInvalidConnection = errors.New(`api_key is required when not connected, and environment must be "sandbox" or "prod"`)
	// errNotStored means the users service accepted the save but didn't keep
	// phone_assistant (e.g. it isn't in its allowed settings keys).
	errNotStored = errors.New("the users service did not store the phone_assistant setting")
	// errStoredProxyInvalid means the stored proxy decrypted to something that
	// isn't a proxy URL. Ours to investigate (Internal), not the caller's input.
	errStoredProxyInvalid = errors.New("stored phone_assistant proxy is not a valid proxy URL")
)

const (
	// clientTimeout bounds each Partner API request; the dashboard is waiting.
	clientTimeout = 15 * time.Second
	// maxProviderPages bounds ListProviders' page walk (100 per page), so a
	// misreported total_pages can't loop forever.
	maxProviderPages = 20

	envSandbox    = string(gooyai.EnvSandbox)
	envProduction = string(gooyai.EnvProduction)
)

// Client is the subset of *gooyai.Client this package uses.
type Client interface {
	ListIncomingCalls(ctx context.Context, f *gooyai.ListCallsFilter) (*gooyai.CallPage, error)
	GetIncomingCall(ctx context.Context, id int64) (*gooyai.IncomingCall, error)
	ListProviders(ctx context.Context, f *gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error)
}

// ClientFactory returns a Client bound to one tenant's API key and
// environment, calling through proxyURL ("" = the server default).
type ClientFactory func(apiKey, env, proxyURL string) Client

// NewClientFactory builds Partner API clients. Clients going through the same
// proxy share one connection pool; defaultProxy (GOOYAI_PROXY) applies to
// clinics without a proxy of their own, and when that's empty too the
// environment's proxy settings do.
func NewClientFactory(baseURL, defaultProxy string) ClientFactory {
	var mu sync.Mutex
	pools := map[string]*http.Client{}
	pool := func(proxy string) *http.Client {
		if proxy == "" {
			proxy = defaultProxy
		}
		mu.Lock()
		defer mu.Unlock()
		c, ok := pools[proxy]
		if !ok {
			c = newProxiedHTTPClient(proxy)
			pools[proxy] = c
		}
		return c
	}
	return func(apiKey, env, proxyURL string) Client {
		return gooyai.New(apiKey,
			gooyai.WithBaseURL(baseURL),
			gooyai.WithEnv(gooyai.Env(env)),
			gooyai.WithHTTPClient(pool(proxyURL)),
		)
	}
}

// NewClientFactoryChecked is NewClientFactory after validating defaultProxy,
// so a malformed GOOYAI_PROXY stops startup instead of silently going direct.
func NewClientFactoryChecked(baseURL, defaultProxy string) (ClientFactory, error) {
	if _, err := parseProxy(defaultProxy); err != nil {
		return nil, fmt.Errorf("GOOYAI_PROXY: %w", err)
	}
	return NewClientFactory(baseURL, defaultProxy), nil
}

// ConnectInput is what a clinic saves from the dashboard.
type ConnectInput struct {
	// APIKey may be empty when already connected, to keep the stored key.
	APIKey      string
	Environment string
	// ProxyURL is optional; empty = the server default. A maskedPassword (as
	// GetConnection shows it) keeps the stored proxy's password.
	ProxyURL string
}

// Service reads a clinic's Gooyai data and manages its connection. tenant is
// the clinic's organizer uuid, already resolved from the caller's identity by
// the handler.
type Service interface {
	ListIncomingCalls(ctx context.Context, tenant string, f *gooyai.ListCallsFilter) (*gooyai.CallPage, error)
	GetIncomingCall(ctx context.Context, tenant string, id int64) (*gooyai.IncomingCall, error)
	ListProviders(ctx context.Context, tenant string) ([]gooyai.ServiceProvider, error)

	// GetConnection returns nil, nil when the clinic isn't connected.
	GetConnection(ctx context.Context, tenant string) (*Connection, error)
	// Connect checks the key (through the proxy) against Gooyai, then stores
	// key and proxy encrypted, replacing any existing connection.
	Connect(ctx context.Context, tenant string, in *ConnectInput) (*Connection, error)
	Disconnect(ctx context.Context, tenant string) error
}

type service struct {
	repo      Repository
	sealer    *Sealer
	newClient ClientFactory
}

// NewService returns a Service. A nil sealer (GOOYAI_ENCRYPTION_KEY unset)
// keeps the service running but every key operation then fails with
// ErrEncryptionNotConfigured.
func NewService(repo Repository, sealer *Sealer, newClient ClientFactory) Service {
	return &service{repo: repo, sealer: sealer, newClient: newClient}
}

func (s *service) client(ctx context.Context, tenant string) (Client, error) {
	conn, err := s.repo.Get(ctx, tenant)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, ErrNotConnected
	}
	key, proxy, err := s.open(conn)
	if err != nil {
		return nil, err
	}
	return s.newClient(key, conn.Environment, proxy), nil
}

// Fields a sealed value can belong to, bound into its ciphertext (see
// phoneAssistantAAD).
const (
	fieldAPIKey   = "api_key"
	fieldProxyURL = "proxy_url"
)

// phoneAssistantAAD names what a sealed value is and whose it is. The users
// service lets a clinic edit its own settings, so without this binding
// key_ciphertext copied into proxy_ciphertext would open as the "proxy" and
// GetConnection would hand the API key back to the browser.
func phoneAssistantAAD(field, tenant string) string {
	return "phone_assistant/" + field + "/" + tenant
}

// open decrypts a stored connection's key and proxy ("" when it has none).
// A proxy that opens but isn't a valid proxy URL is refused, never returned.
func (s *service) open(conn *Connection) (key, proxy string, err error) {
	if key, err = s.sealer.Open(conn.APIKeySealed, phoneAssistantAAD(fieldAPIKey, conn.OrganizerUUID)); err != nil {
		return "", "", err
	}
	if proxy, err = s.openProxy(conn); err != nil {
		return "", "", err
	}
	return key, proxy, nil
}

// openProxy decrypts and validates the stored proxy ("" when there is none).
func (s *service) openProxy(conn *Connection) (string, error) {
	if len(conn.ProxySealed) == 0 {
		return "", nil
	}
	proxy, err := s.sealer.Open(conn.ProxySealed, phoneAssistantAAD(fieldProxyURL, conn.OrganizerUUID))
	if err != nil {
		return "", err
	}
	if _, err := parseProxy(proxy); err != nil {
		return "", errStoredProxyInvalid
	}
	return proxy, nil
}

// withRedactedProxy fills conn.ProxyRedacted, what the dashboard may see of
// the proxy. conn may be nil (not connected).
func (s *service) withRedactedProxy(conn *Connection) (*Connection, error) {
	if conn == nil || len(conn.ProxySealed) == 0 {
		return conn, nil
	}
	proxy, err := s.openProxy(conn)
	if err != nil {
		return nil, err
	}
	conn.ProxyRedacted = redactProxy(proxy)
	return conn, nil
}

func (s *service) ListIncomingCalls(ctx context.Context, tenant string, f *gooyai.ListCallsFilter) (*gooyai.CallPage, error) {
	c, err := s.client(ctx, tenant)
	if err != nil {
		return nil, err
	}
	return c.ListIncomingCalls(ctx, f)
}

func (s *service) GetIncomingCall(ctx context.Context, tenant string, id int64) (*gooyai.IncomingCall, error) {
	c, err := s.client(ctx, tenant)
	if err != nil {
		return nil, err
	}
	return c.GetIncomingCall(ctx, id)
}

// ListProviders returns every provider: a clinic has few, and the dashboard
// needs the whole list to match them against its organizers.
func (s *service) ListProviders(ctx context.Context, tenant string) ([]gooyai.ServiceProvider, error) {
	c, err := s.client(ctx, tenant)
	if err != nil {
		return nil, err
	}

	var all []gooyai.ServiceProvider
	for page := 1; page <= maxProviderPages; page++ {
		res, err := c.ListProviders(ctx, &gooyai.ListProvidersFilter{
			PageQuery: gooyai.PageQuery{Page: page, Limit: gooyai.MaxLimit},
		})
		if err != nil {
			return nil, err
		}
		all = append(all, res.Providers...)
		if page >= res.Pagination.TotalPages {
			break
		}
	}
	return all, nil
}

func (s *service) GetConnection(ctx context.Context, tenant string) (*Connection, error) {
	conn, err := s.repo.Get(ctx, tenant)
	if err != nil {
		return nil, err
	}
	return s.withRedactedProxy(conn)
}

func (s *service) Connect(ctx context.Context, tenant string, in *ConnectInput) (*Connection, error) {
	apiKey := strings.TrimSpace(in.APIKey)
	proxy := strings.TrimSpace(in.ProxyURL)
	env := in.Environment
	if env == "" {
		env = envSandbox
	}
	if env != envSandbox && env != envProduction {
		return nil, ErrInvalidConnection
	}
	if s.sealer == nil {
		return nil, ErrEncryptionNotConfigured
	}

	apiKey, proxy, err := s.fillFromStored(ctx, tenant, apiKey, proxy)
	if err != nil {
		return nil, err
	}
	if _, err := parseProxy(proxy); err != nil {
		return nil, err
	}

	// One cheap authenticated read proves the key works for this environment,
	// through this proxy, before it's saved, so a typo fails here rather than
	// later as "calls unavailable".
	_, err = s.newClient(apiKey, env, proxy).ListProviders(ctx, &gooyai.ListProvidersFilter{
		PageQuery: gooyai.PageQuery{Page: 1, Limit: 1},
	})
	if errors.Is(err, gooyai.ErrUnauthorized) {
		return nil, ErrKeyRejected
	}
	if err != nil {
		return nil, err
	}

	return s.store(ctx, tenant, apiKey, env, proxy)
}

// fillFromStored completes a save from the stored connection: a blank key
// keeps the stored key, and a masked proxy password keeps the stored one. A
// blank key with nothing stored is ErrInvalidConnection.
func (s *service) fillFromStored(ctx context.Context, tenant, apiKey, proxy string) (key, proxyURL string, err error) {
	if apiKey != "" && !strings.Contains(proxy, maskedPassword) {
		return apiKey, proxy, nil
	}
	conn, err := s.repo.Get(ctx, tenant)
	if err != nil {
		return "", "", err
	}
	if conn == nil {
		if apiKey == "" {
			return "", "", ErrInvalidConnection
		}
		return apiKey, proxy, nil
	}
	storedKey, storedProxy, err := s.open(conn)
	if err != nil {
		return "", "", err
	}
	if apiKey == "" {
		apiKey = storedKey
	}
	return apiKey, restoreProxyPassword(proxy, storedProxy), nil
}

// store seals the key and proxy and saves them as the tenant's connection.
func (s *service) store(ctx context.Context, tenant, apiKey, env, proxy string) (*Connection, error) {
	sealed, err := s.sealer.Seal(apiKey, phoneAssistantAAD(fieldAPIKey, tenant))
	if err != nil {
		return nil, err
	}
	var sealedProxy []byte
	if proxy != "" {
		if sealedProxy, err = s.sealer.Seal(proxy, phoneAssistantAAD(fieldProxyURL, tenant)); err != nil {
			return nil, err
		}
	}
	written := &Connection{
		OrganizerUUID: tenant,
		APIKeySealed:  sealed,
		KeyLast4:      last4(apiKey),
		Environment:   env,
		ProxySealed:   sealedProxy,
		UpdatedAt:     time.Now().UTC(),
	}
	if err := s.repo.Upsert(ctx, written); err != nil {
		return nil, err
	}
	// Read it back: the users service silently drops settings keys it doesn't
	// allow, so a save that didn't land must fail here, not look connected —
	// including when an older connection is still there.
	conn, err := s.repo.Get(ctx, tenant)
	if err != nil {
		return nil, err
	}
	if !sameConnection(conn, written) {
		return nil, errNotStored
	}
	return s.withRedactedProxy(conn)
}

// sameConnection reports whether stored is the write just made. Each Seal uses
// a fresh random nonce, so matching ciphertext proves it's this write and not
// an older connection left in place.
func sameConnection(stored, written *Connection) bool {
	return stored != nil &&
		bytes.Equal(stored.APIKeySealed, written.APIKeySealed) &&
		bytes.Equal(stored.ProxySealed, written.ProxySealed) &&
		stored.Environment == written.Environment &&
		stored.KeyLast4 == written.KeyLast4
}

func (s *service) Disconnect(ctx context.Context, tenant string) error {
	return s.repo.Delete(ctx, tenant)
}

// last4 is what the dashboard shows to identify a key ("…ab12"). A key too
// short for 4 characters to be a small fraction of it gets no hint at all.
func last4(key string) string {
	if len(key) < 12 {
		return ""
	}
	return key[len(key)-4:]
}
