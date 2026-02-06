package dto

// AuthProvider represents an OAuth provider.
// @Description OAuth provider information
type AuthProvider struct {
	ID      string `json:"id" example:"github"`
	Name    string `json:"name" example:"GitHub"`
	AuthURL string `json:"auth_url" example:"https://github.com/login/oauth/authorize"`
	Enabled bool   `json:"enabled" example:"true"`
}

// AuthProviderResponse represents a list of exchange rates
//
//	@Description	List of exchange rates
type AuthProviderResponse []AuthProvider
