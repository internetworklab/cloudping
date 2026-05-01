package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// More on https://developers.google.com/identity/openid-connect/openid-connect#userinfoproduct
type GoogleUserInfoResponse struct {
	Sub           string `json:"sub"`
	Id            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

func GetGoogleProfileByToken(ctx context.Context, token string) (*GoogleUserInfoResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create HTTP request to obtain profile of current Google user: %+v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/json")

	cli := http.DefaultClient
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to obtain profile of current Google user: %v", err)
	}
	defer resp.Body.Close()

	profileObject := new(GoogleUserInfoResponse)
	if err := json.NewDecoder(resp.Body).Decode(profileObject); err != nil {
		return nil, fmt.Errorf("Failed to obtain profile of current Google user: %v", err)
	}
	return profileObject, nil
}
