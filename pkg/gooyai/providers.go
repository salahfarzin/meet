package gooyai

import (
	"context"
	"strconv"
)

// ListProviders returns one page of the tenant's service providers; a nil
// filter means no filters.
func (c *Client) ListProviders(ctx context.Context, f *ListProvidersFilter) (*ProviderPage, error) {
	if f == nil {
		f = &ListProvidersFilter{}
	}
	q := query{}.page(f.PageQuery).
		bool("is_active", f.IsActive).
		str("search", f.Search)

	var page ProviderPage
	if err := c.get(ctx, "/providers", q.values(), &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// GetProvider returns one provider.
func (c *Client) GetProvider(ctx context.Context, id int64) (*ServiceProvider, error) {
	var p ServiceProvider
	if err := c.get(ctx, providerPath(id), nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ProviderAttributes returns the attribute values set on one provider.
func (c *Client) ProviderAttributes(ctx context.Context, providerID int64) ([]Attribute, error) {
	var attrs []Attribute
	if err := c.get(ctx, providerPath(providerID)+"/attributes", nil, &attrs); err != nil {
		return nil, err
	}
	return attrs, nil
}

// AttributeCatalog returns every attribute and its allowed values; their ids
// are what AttributeSelection refers to.
func (c *Client) AttributeCatalog(ctx context.Context) ([]CatalogAttribute, error) {
	var attrs []CatalogAttribute
	if err := c.get(ctx, "/service-provider-attributes", nil, &attrs); err != nil {
		return nil, err
	}
	return attrs, nil
}

func providerPath(id int64) string {
	return "/providers/" + strconv.FormatInt(id, 10)
}
