// Package gooyai exposes a clinic's Gooyai (AI phone receptionist) data to
// the dashboard. The Partner API client itself lives in pkg/gooyai; this
// package owns which clinic's key is used and how results map to our proto.
package gooyai

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/salahfarzin/meet/pkg/gooyai"
)

// ErrNotConnected means the clinic has no Gooyai API key configured.
var ErrNotConnected = errors.New("clinic is not connected to Gooyai")

const (
	// clientTimeout bounds each Partner API request; the dashboard is waiting.
	clientTimeout = 15 * time.Second
	// maxProviderPages bounds ListProviders' page walk (100 per page), so a
	// misreported total_pages can't loop forever.
	maxProviderPages = 20
)

// Client is the subset of *gooyai.Client this package uses.
type Client interface {
	ListIncomingCalls(ctx context.Context, f *gooyai.ListCallsFilter) (*gooyai.CallPage, error)
	GetIncomingCall(ctx context.Context, id int64) (*gooyai.IncomingCall, error)
	ListProviders(ctx context.Context, f *gooyai.ListProvidersFilter) (*gooyai.ProviderPage, error)
}

// ClientFactory returns a Client bound to one tenant's API key.
type ClientFactory func(apiKey string) Client

// NewClientFactory builds Partner API clients that share one connection pool.
func NewClientFactory(baseURL, env string) ClientFactory {
	shared := &http.Client{Timeout: clientTimeout}
	return func(apiKey string) Client {
		return gooyai.New(apiKey,
			gooyai.WithBaseURL(baseURL),
			gooyai.WithEnv(gooyai.Env(env)),
			gooyai.WithHTTPClient(shared),
		)
	}
}

// Service reads a clinic's Gooyai data. tenant is the clinic's organizer
// uuid, already resolved from the caller's identity by the handler.
type Service interface {
	ListIncomingCalls(ctx context.Context, tenant string, f *gooyai.ListCallsFilter) (*gooyai.CallPage, error)
	GetIncomingCall(ctx context.Context, tenant string, id int64) (*gooyai.IncomingCall, error)
	ListProviders(ctx context.Context, tenant string) ([]gooyai.ServiceProvider, error)
}

type service struct {
	keys      map[string]string
	newClient ClientFactory
}

// NewService returns a Service. keys maps organizer uuid to API key; a clinic
// without an entry gets ErrNotConnected.
func NewService(keys map[string]string, newClient ClientFactory) Service {
	return &service{keys: keys, newClient: newClient}
}

func (s *service) client(tenant string) (Client, error) {
	key, ok := s.keys[tenant]
	if !ok || key == "" {
		return nil, ErrNotConnected
	}
	return s.newClient(key), nil
}

func (s *service) ListIncomingCalls(ctx context.Context, tenant string, f *gooyai.ListCallsFilter) (*gooyai.CallPage, error) {
	c, err := s.client(tenant)
	if err != nil {
		return nil, err
	}
	return c.ListIncomingCalls(ctx, f)
}

func (s *service) GetIncomingCall(ctx context.Context, tenant string, id int64) (*gooyai.IncomingCall, error) {
	c, err := s.client(tenant)
	if err != nil {
		return nil, err
	}
	return c.GetIncomingCall(ctx, id)
}

// ListProviders returns every provider: a clinic has few, and the dashboard
// needs the whole list to match them against its organizers.
func (s *service) ListProviders(ctx context.Context, tenant string) ([]gooyai.ServiceProvider, error) {
	c, err := s.client(tenant)
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
