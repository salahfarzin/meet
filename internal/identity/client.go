package identity

import "context"

type IdentityFilter struct {
	FirstName, LastName, NationalID, Mobile string
}

type Identity struct {
	UUID, FirstName, LastName, NationalCode, Mobile, Name string
}

type Clinic struct {
	UUID, Name string
}

type Client interface {
	Search(ctx context.Context, f IdentityFilter) ([]string, error)
	GetByUUIDs(ctx context.Context, uuids []string) (map[string]Identity, error)
	ListClinics(ctx context.Context) ([]Clinic, error)
}

type ctxKey string

const bearerKey ctxKey = "identity-bearer"

func WithBearer(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, bearerKey, token)
}

func bearerFrom(ctx context.Context) string {
	if v, ok := ctx.Value(bearerKey).(string); ok {
		return v
	}
	return ""
}
