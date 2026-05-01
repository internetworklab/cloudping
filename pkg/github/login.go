package github

type GithubTokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
}

// More on https://docs.github.com/en/rest/users/users?apiVersion=2022-11-28
type GithubProfileResponse struct {
	Login     string `json:"login"`
	Id        *int   `json:"id,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	HTMLURL   string `json:"html_url,omitempty"`
	Type      string `json:"type,omitempty"`
	Name      string `json:"name,omitempty"`
}
