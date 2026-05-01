package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func GetGithubProfileByToken(ctx context.Context, token string) (*GithubProfileResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create HTTP request to obtain profile of current Github user: %+v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/json")

	cli := http.DefaultClient
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to obtain profile of current Github user: %v", err)
	}
	defer resp.Body.Close()

	profileObject := new(GithubProfileResponse)
	if err := json.NewDecoder(resp.Body).Decode(profileObject); err != nil {
		return nil, fmt.Errorf("Failed to obtain profile of current Github user: %v", err)
	}
	return profileObject, nil
}
