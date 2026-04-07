// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package auth

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/larksuite/cli/internal/core"
)

// AuthorizationCodeFlowResult is the result of the authorization code flow.
type AuthorizationCodeFlowResult struct {
	OK      bool
	Token   *DeviceFlowTokenData
	Error   string
	Message string
}

// generateState generates a random state parameter for CSRF protection.
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// StartAuthorizationCodeFlow prints the authorization URL and waits for the user
// to paste the code (or full callback URL) from the browser.
func StartAuthorizationCodeFlow(
	ctx context.Context,
	httpClient *http.Client,
	appId, appSecret string,
	brand core.LarkBrand,
	scope string,
	redirectURI string,
	stdin io.Reader,
	out io.Writer,
) *AuthorizationCodeFlowResult {
	if !strings.Contains(scope, "offline_access") {
		if scope != "" {
			scope = scope + " offline_access"
		} else {
			scope = "offline_access"
		}
	}

	// Authorization Code Flow is limited to 50 scopes per request
	const maxScopes = 50
	scopes := strings.Fields(scope)
	if len(scopes) > maxScopes {
		return &AuthorizationCodeFlowResult{
			OK:      false,
			Error:   "too_many_scopes",
			Message: fmt.Sprintf("%d scopes requested, authorization code flow allows at most %d; use --domain to narrow the scope", len(scopes), maxScopes),
		}
	}

	endpoints := ResolveOAuthEndpoints(brand)
	state, err := generateState()
	if err != nil {
		return &AuthorizationCodeFlowResult{OK: false, Error: "internal_error", Message: fmt.Sprintf("failed to generate state: %v", err)}
	}

	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		endpoints.Authorize,
		url.QueryEscape(appId),
		url.QueryEscape(redirectURI),
		url.QueryEscape(scope),
		url.QueryEscape(state),
	)

	fmt.Fprintf(out, "Open the following URL in your browser to authorize:\n\n  %s\n\n", authURL)
	fmt.Fprintf(out, "After authorization, paste the 'code' value from the callback URL and press Enter:\n> ")

	// Read code from stdin
	scanner := bufio.NewScanner(stdin)
	var input string
	doneCh := make(chan string, 1)
	go func() {
		if scanner.Scan() {
			doneCh <- strings.TrimSpace(scanner.Text())
		} else {
			doneCh <- ""
		}
	}()

	select {
	case input = <-doneCh:
	case <-ctx.Done():
		return &AuthorizationCodeFlowResult{OK: false, Error: "cancelled", Message: "cancelled"}
	}

	if input == "" {
		return &AuthorizationCodeFlowResult{OK: false, Error: "no_code", Message: "no code entered"}
	}

	// Accept either a bare code or the full callback URL
	code := extractCode(input, state)
	if code == "" {
		return &AuthorizationCodeFlowResult{OK: false, Error: "invalid_input", Message: fmt.Sprintf("could not extract code from: %s", input)}
	}

	return exchangeCodeForToken(ctx, httpClient, appId, appSecret, code, redirectURI, endpoints)
}

// extractCode extracts the authorization code from either a bare code string
// or a full callback URL (e.g. https://example.com/callback?code=xxx&state=yyy).
func extractCode(input, expectedState string) string {
	if !strings.Contains(input, "://") {
		// Bare code
		return input
	}
	u, err := url.Parse(input)
	if err != nil {
		return ""
	}
	if st := u.Query().Get("state"); st != "" && st != expectedState {
		return ""
	}
	return u.Query().Get("code")
}

// exchangeCodeForToken exchanges the authorization code for an access token.
func exchangeCodeForToken(
	ctx context.Context,
	httpClient *http.Client,
	appId, appSecret, code, redirectURI string,
	endpoints OAuthEndpoints,
) *AuthorizationCodeFlowResult {
	body, err := json.Marshal(map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     appId,
		"client_secret": appSecret,
		"code":          code,
		"redirect_uri":  redirectURI,
	})
	if err != nil {
		return &AuthorizationCodeFlowResult{OK: false, Error: "internal_error", Message: fmt.Sprintf("failed to build request: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoints.Token, strings.NewReader(string(body)))
	if err != nil {
		return &AuthorizationCodeFlowResult{OK: false, Error: "internal_error", Message: fmt.Sprintf("failed to create request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := httpClient.Do(req)
	if err != nil {
		return &AuthorizationCodeFlowResult{OK: false, Error: "network_error", Message: fmt.Sprintf("failed to exchange code: %v", err)}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &AuthorizationCodeFlowResult{OK: false, Error: "internal_error", Message: fmt.Sprintf("failed to read response: %v", err)}
	}

	var data map[string]interface{}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return &AuthorizationCodeFlowResult{OK: false, Error: "internal_error", Message: fmt.Sprintf("failed to parse response: %v", err)}
	}

	// API 用 code != 0 表示失败
	if apiCode := getInt(data, "code", 0); apiCode != 0 {
		errDesc := getStr(data, "error_description")
		if errDesc == "" {
			errDesc = getStr(data, "error")
		}
		if errDesc == "" {
			errDesc = fmt.Sprintf("error code %d", apiCode)
		}
		return &AuthorizationCodeFlowResult{OK: false, Error: getStr(data, "error"), Message: errDesc}
	}

	accessToken := getStr(data, "access_token")
	if accessToken == "" {
		return &AuthorizationCodeFlowResult{OK: false, Error: "internal_error", Message: "no access token in response"}
	}

	refreshToken := getStr(data, "refresh_token")
	tokenExpiresIn := getInt(data, "expires_in", 7200)
	refreshExpiresIn := getInt(data, "refresh_token_expires_in", 604800)
	if refreshToken == "" {
		refreshExpiresIn = tokenExpiresIn
	}

	return &AuthorizationCodeFlowResult{
		OK: true,
		Token: &DeviceFlowTokenData{
			AccessToken:      accessToken,
			RefreshToken:     refreshToken,
			ExpiresIn:        tokenExpiresIn,
			RefreshExpiresIn: refreshExpiresIn,
			Scope:            getStr(data, "scope"),
		},
	}
}
