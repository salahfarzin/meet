package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type httpClient struct {
	baseURL string
	hc      *http.Client
}

func NewHTTPClient(baseURL string, hc *http.Client) Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &httpClient{baseURL: baseURL, hc: hc}
}

type usersEnvelope struct {
	Items  []userItem `json:"items"`
	Return *struct {
		Items []userItem `json:"items"`
	} `json:"return"`
}

type userItem struct {
	UUID         string `json:"uuid"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	NationalCode string `json:"national_code"`
	Mobile       string `json:"mobile"`
	Name         string `json:"name"`
}

func (c *httpClient) items(ctx context.Context, q url.Values) ([]userItem, error) {
	u := c.baseURL + "/users?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	if tok := bearerFrom(ctx); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity /users status %d", resp.StatusCode)
	}
	var env usersEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	if env.Return != nil {
		return env.Return.Items, nil
	}
	return env.Items, nil
}

func addFilter(q url.Values, i int, field, value, match string) {
	q.Set(fmt.Sprintf("filters[%d][field]", i), field)
	q.Set(fmt.Sprintf("filters[%d][value]", i), value)
	if match != "" {
		q.Set(fmt.Sprintf("match_mode[%s]", field), match)
	}
}

func (c *httpClient) Search(ctx context.Context, f IdentityFilter) ([]string, error) {
	q := url.Values{}
	q.Set("limit", "1000")
	i := 0
	for field, val := range map[string]string{
		"first_name": f.FirstName, "last_name": f.LastName,
		"national_code": f.NationalID, "mobile": f.Mobile,
	} {
		if val != "" {
			addFilter(q, i, field, val, "contains")
			i++
		}
	}
	items, err := c.items(ctx, q)
	if err != nil {
		return nil, err
	}
	uuids := make([]string, 0, len(items))
	for _, it := range items {
		uuids = append(uuids, it.UUID)
	}
	return uuids, nil
}

func (c *httpClient) GetByUUIDs(ctx context.Context, uuids []string) (map[string]Identity, error) {
	out := map[string]Identity{}
	if len(uuids) == 0 {
		return out, nil
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(len(uuids)))
	for _, id := range uuids {
		q.Add("filters[0][value][]", id)
	}
	q.Set("filters[0][field]", "uuid")
	q.Set("match_mode[uuid]", "in")
	items, err := c.items(ctx, q)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		out[it.UUID] = Identity{
			UUID: it.UUID, FirstName: it.FirstName, LastName: it.LastName,
			NationalCode: it.NationalCode, Mobile: it.Mobile,
		}
	}
	return out, nil
}

func (c *httpClient) ListClinics(ctx context.Context) ([]Clinic, error) {
	q := url.Values{}
	q.Set("limit", "1000")
	addFilter(q, 0, "center", "1", "")
	items, err := c.items(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]Clinic, 0, len(items))
	for _, it := range items {
		name := it.Name
		if name == "" {
			name = it.FirstName + " " + it.LastName
		}
		out = append(out, Clinic{UUID: it.UUID, Name: name})
	}
	return out, nil
}
