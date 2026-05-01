package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// FetchUserInfo retrieves the user profile from the OIDC userinfo endpoint.
func FetchUserInfo(ctx context.Context, endpoint, accessToken string) (*UserInfoResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create userinfo request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request returned status %d", resp.StatusCode)
	}

	userinfo := new(UserInfoResponse)
	if err := json.NewDecoder(resp.Body).Decode(userinfo); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo response: %w", err)
	}
	return userinfo, nil
}
