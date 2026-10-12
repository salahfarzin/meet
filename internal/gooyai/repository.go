package gooyai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"
)

// settingsKey is where a clinic's connection lives in its user settings.
const settingsKey = "phone_assistant"

var (
	// ErrUsersServiceNotConfigured means USERS_SERVICE is unset, so there is
	// nowhere to read or store a clinic's connection.
	ErrUsersServiceNotConfigured = errors.New("users service is not configured")
	// errNoCallerToken means the request carried no bearer token to call the
	// users service with.
	errNoCallerToken = errors.New("no caller bearer token to call the users service with")
)

// Connection is a clinic's stored Gooyai connection. APIKeySealed is the
// encrypted key (see Sealer); the plaintext is never kept on this struct.
type Connection struct {
	OrganizerUUID string
	APIKeySealed  []byte
	KeyLast4      string
	Environment   string
	// ProxySealed is the encrypted proxy URL (it may hold credentials); nil =
	// no proxy of the clinic's own.
	ProxySealed []byte
	UpdatedAt   time.Time

	// ProxyRedacted is the proxy as the dashboard may see it (password
	// masked). Filled by the Service, never stored.
	ProxyRedacted string
}

// Repository stores one Connection per organizer.
type Repository interface {
	// Get returns nil, nil when the organizer has no connection.
	Get(ctx context.Context, organizerUUID string) (*Connection, error)
	Upsert(ctx context.Context, c *Connection) error
	Delete(ctx context.Context, organizerUUID string) error
}

// storedConnection is a Connection as kept in the user's settings. The key is
// sealed before it gets here, so the settings — which the users service hands
// to the browser on GET /users/{uuid} — only ever carry ciphertext.
type storedConnection struct {
	KeyCiphertext   string    `json:"key_ciphertext"`
	KeyLast4        string    `json:"key_last4"`
	Environment     string    `json:"environment"`
	ProxyCiphertext string    `json:"proxy_ciphertext,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// usersRepository keeps each clinic's connection in its user settings
// (psychometrist's users_info.center_details, "settings" on /users/{uuid}),
// under phone_assistant. It calls the users service as the caller, forwarding
// their own bearer token, so the users service's own rules decide whose
// settings may be read or changed.
type usersRepository struct {
	baseURL string // e.g. https://api.psychometrist.local/v1
	client  *http.Client
}

// NewUsersRepository returns a Repository backed by the users service at
// baseURL. An empty baseURL fails every call with ErrUsersServiceNotConfigured.
func NewUsersRepository(baseURL string, client *http.Client) Repository {
	return &usersRepository{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

func (repo *usersRepository) Get(ctx context.Context, organizerUUID string) (*Connection, error) {
	uuid, err := repo.resolve(ctx, organizerUUID)
	if err != nil {
		return nil, err
	}

	var user struct {
		Settings map[string]json.RawMessage `json:"settings"`
	}
	if err := repo.do(ctx, http.MethodGet, "/users/"+url.PathEscape(uuid), nil, &user); err != nil {
		return nil, err
	}

	raw := user.Settings[settingsKey]
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil //nolint:nilnil // nil = not connected, distinct from a failure
	}
	var stored storedConnection
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, fmt.Errorf("decode %s settings: %w", settingsKey, err)
	}
	sealed, err := base64.StdEncoding.DecodeString(stored.KeyCiphertext)
	if err != nil || len(sealed) == 0 {
		return nil, fmt.Errorf("decode %s key ciphertext: %w", settingsKey, err)
	}
	var sealedProxy []byte
	if stored.ProxyCiphertext != "" {
		if sealedProxy, err = base64.StdEncoding.DecodeString(stored.ProxyCiphertext); err != nil {
			return nil, fmt.Errorf("decode %s proxy ciphertext: %w", settingsKey, err)
		}
	}
	return &Connection{
		OrganizerUUID: organizerUUID,
		APIKeySealed:  sealed,
		KeyLast4:      stored.KeyLast4,
		Environment:   stored.Environment,
		ProxySealed:   sealedProxy,
		UpdatedAt:     stored.UpdatedAt,
	}, nil
}

func (repo *usersRepository) Upsert(ctx context.Context, c *Connection) error {
	stored := &storedConnection{
		KeyCiphertext: base64.StdEncoding.EncodeToString(c.APIKeySealed),
		KeyLast4:      c.KeyLast4,
		Environment:   c.Environment,
		UpdatedAt:     c.UpdatedAt.UTC(),
	}
	if len(c.ProxySealed) > 0 {
		stored.ProxyCiphertext = base64.StdEncoding.EncodeToString(c.ProxySealed)
	}
	return repo.put(ctx, c.OrganizerUUID, stored)
}

// Delete clears phone_assistant to null; the users service merges settings
// key by key, so the clinic's other settings are left alone.
func (repo *usersRepository) Delete(ctx context.Context, organizerUUID string) error {
	return repo.put(ctx, organizerUUID, nil)
}

func (repo *usersRepository) put(ctx context.Context, organizerUUID string, stored *storedConnection) error {
	uuid, err := repo.resolve(ctx, organizerUUID)
	if err != nil {
		return err
	}
	body := map[string]any{"settings": map[string]any{settingsKey: stored}}
	return repo.do(ctx, http.MethodPut, "/users/"+url.PathEscape(uuid), body, nil)
}

// resolve maps the AUTH_DISABLED placeholder tenant to the caller's real uuid,
// since the users service only knows real users.
func (repo *usersRepository) resolve(ctx context.Context, organizerUUID string) (string, error) {
	if organizerUUID != localTenant {
		return organizerUUID, nil
	}
	var me struct {
		UUID string `json:"uuid"`
	}
	if err := repo.do(ctx, http.MethodGet, "/me", nil, &me); err != nil {
		return "", err
	}
	if me.UUID == "" {
		return "", errors.New("users service /me returned no uuid")
	}
	return me.UUID, nil
}

// do calls the users service with the caller's bearer token and decodes the
// "return" member of its {success, return} envelope into out, when out is set.
func (repo *usersRepository) do(ctx context.Context, method, path string, body, out any) error {
	req, err := repo.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}

	resp, err := repo.client.Do(req)
	if err != nil {
		return fmt.Errorf("users service %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// The body is the users service's own error list; it's for our logs,
		// never the dashboard (toStatus maps this to a generic Internal).
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("users service %s %s: status %d: %s", method, path, resp.StatusCode, msg)
	}
	if out == nil {
		return nil
	}
	envelope := struct {
		Return json.RawMessage `json:"return"`
	}{}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("users service %s %s: decode: %w", method, path, err)
	}
	if err := json.Unmarshal(envelope.Return, out); err != nil {
		return fmt.Errorf("users service %s %s: decode return: %w", method, path, err)
	}
	return nil
}

// newRequest builds a users service request authenticated as the caller.
func (repo *usersRepository) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	if repo.baseURL == "" {
		return nil, ErrUsersServiceNotConfigured
	}
	token, err := callerToken(ctx)
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader = http.NoBody
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, repo.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// callerToken is the caller's Authorization header, which grpc-gateway
// forwards as "authorization" metadata.
func callerToken(ctx context.Context) (string, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if v := md.Get("authorization"); len(v) > 0 && strings.TrimSpace(v[0]) != "" {
		return v[0], nil
	}
	return "", errNoCallerToken
}
