package types

type ForgetPasswordRequest struct {
	Username string `json:"username"`
	Phone    string `json:"phone"`
}
