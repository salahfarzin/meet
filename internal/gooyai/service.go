// Package gooyai exposes a clinic's Gooyai (AI phone receptionist) data to
// the dashboard. The Partner API client itself lives in pkg/gooyai; this
// package owns each clinic's connection (its encrypted key) and how results
// map to our proto.
package gooyai

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/salahfarzin/meet/pkg/gooyai"
)

var (
	// ErrNotConnected means the clinic has no Gooyai connection.
	ErrNotConnected = errors.New("clinic is not connected to Gooyai")
	// ErrKeyRejected means Gooyai refused the key while connecting.
	ErrKeyRejected = errors.New("gooyai rejected this API key")
	// ErrInvalidConnection covers a blank key or an unknown environment.
	ErrInvalidConnection = errors.New(`api_key is required and environment must be "sandbox" or "prod"`)
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

// ClientFactory returns a Client bound to one tenant's API key and environment.
type ClientFactory func(apiKey, env string) Client

// NewClientFactory builds Partner API clients that share one connection pool.
func NewClientFactory(baseURL string) ClientFactory {
	shared := &http.Client{Timeout: clientTimeout}
	return func(apiKey, env string) Client {
		return gooyai.New(apiKey,
			gooyai.WithBaseURL(baseURL),
			gooyai.WithEnv(gooyai.Env(env)),
			gooyai.WithHTTPClient(shared),
		)
	}
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
	// Connect checks apiKey against Gooyai, then stores it encrypted,
	// replacing any existing connection.
	Connect(ctx context.Context, tenant, apiKey, env string) (*Connection, error)
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
	key, err := s.sealer.Open(conn.APIKeySealed)
	if err != nil {
		return nil, err
	}
	return s.newClient(key, conn.Environment), nil
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
	return s.repo.Get(ctx, tenant)
}

func (s *service) Connect(ctx context.Context, tenant, apiKey, env string) (*Connection, error) {
	apiKey = strings.TrimSpace(apiKey)
	if env == "" {
		env = envSandbox
	}
	if apiKey == "" || (env != envSandbox && env != envProduction) {
		return nil, ErrInvalidConnection
	}
	if s.sealer == nil {
		return nil, ErrEncryptionNotConfigured
	}

	// One cheap authenticated read proves the key works for this environment
	// before it's saved, so a typo fails here rather than later as "calls
	// unavailable".
	_, err := s.newClient(apiKey, env).ListProviders(ctx, &gooyai.ListProvidersFilter{
		PageQuery: gooyai.PageQuery{Page: 1, Limit: 1},
	})
	if errors.Is(err, gooyai.ErrUnauthorized) {
		return nil, ErrKeyRejected
	}
	if err != nil {
		return nil, err
	}

	sealed, err := s.sealer.Seal(apiKey)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Upsert(ctx, &Connection{
		OrganizerUUID: tenant,
		APIKeySealed:  sealed,
		KeyLast4:      last4(apiKey),
		Environment:   env,
	}); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, tenant)
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
