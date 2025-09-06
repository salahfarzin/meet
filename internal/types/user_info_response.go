package types

type UserInfoResponse struct {
	FirstName   string  `json:"first_name"`
	LastName    string  `json:"last_name"`
	Email       string  `json:"email"`
	PhoneNumber string  `json:"phone_number"`
	Balance     float64 `json:"balance"`
}
