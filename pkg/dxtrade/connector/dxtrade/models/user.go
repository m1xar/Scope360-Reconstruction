package models

type UserDetailsList struct {
	UserDetails []UserDetails `json:"userDetails"`
}

type UserDetails struct {
	Login    string           `json:"login"`
	Domain   string           `json:"domain"`
	Username string           `json:"username"`
	FullName string           `json:"fullName"`
	Accounts []AccountDetails `json:"accounts"`
}

type AccountDetails struct {
	Owner            string `json:"owner"`
	Account          string `json:"account"`
	BaseCurrency     string `json:"baseCurrency"`
	RegistrationTime Time   `json:"registrationTime"`
	AccountStatus    string `json:"accountStatus"`
	PricingStream    string `json:"pricingStream"`
	IsPositionBased  *Bool  `json:"isPositionBased"`
	PositionBased    *Bool  `json:"positionBased"`
}

func (a AccountDetails) Hedging() bool {
	if a.IsPositionBased != nil {
		return bool(*a.IsPositionBased)
	}
	if a.PositionBased != nil {
		return bool(*a.PositionBased)
	}
	return true
}
