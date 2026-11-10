package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)


func refreshIMAPToken(ctx context.Context, client *http.Client, config *oauth2.Config, current *oauth2.Token) (*oauth2.Token, error) {
	if current.Valid() {
		return current, nil
	}
	if current.RefreshToken == "" {
		return nil, &oauth2.RetrieveError{ErrorCode: "invalid_grant"}
	}
	form := url.Values{"client_id": {config.ClientID}, "grant_type": {"refresh_token"}, "refresh_token": {current.RefreshToken}, "scope": {strings.Join(config.Scopes, " ")}}
	if config.ClientSecret != "" {
		form.Set("client_secret", config.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.Endpoint.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(body) > 1<<20 {
		return nil, fmt.Errorf("OAuth token response too large")
	}
	var data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Error        string `json:"error"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("invalid OAuth token response")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 || data.Error != "" {
		return nil, &oauth2.RetrieveError{Response: res, ErrorCode: data.Error}
	}
	if data.AccessToken == "" || data.ExpiresIn <= 0 || data.ExpiresIn > 86400*365 {
		return nil, fmt.Errorf("OAuth response omitted a valid access token or expiry")
	}
	if data.RefreshToken == "" {
		data.RefreshToken = current.RefreshToken
	}
	return &oauth2.Token{AccessToken: data.AccessToken, RefreshToken: data.RefreshToken, TokenType: data.TokenType, Expiry: time.Now().Add(time.Duration(data.ExpiresIn) * time.Second)}, nil
}
