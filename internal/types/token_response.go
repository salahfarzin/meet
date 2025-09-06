package types

type TokenResponse struct {
	AccesssToken string `json:"access_token"`
	ExpiresAt    int    `json:"expires_at"`
}
