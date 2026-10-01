package models

type AccountInput struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	Provider     string `json:"provider"`
	ClientID     string `json:"clientId"`
	RefreshToken string `json:"refreshToken"`
	IncomingHost string `json:"incomingHost"`
	IncomingPort int    `json:"incomingPort"`
	TLSMode      string `json:"tlsMode"`
	Folder       string `json:"folder"`
	ProxyID      *uint  `json:"proxyId"`
}

type AccountUpdateInput struct {
	Email        *string `json:"email"`
	Password     *string `json:"password"`
	Provider     *string `json:"provider"`
	ClientID     *string `json:"clientId"`
	RefreshToken *string `json:"refreshToken"`
	IncomingHost *string `json:"incomingHost"`
	IncomingPort *int    `json:"incomingPort"`
	TLSMode      *string `json:"tlsMode"`
	Folder       *string `json:"folder"`
	ProxyID      *uint   `json:"proxyId"`
	Enabled      *bool   `json:"enabled"`
	SyncEnabled  *bool   `json:"syncEnabled"`
}
