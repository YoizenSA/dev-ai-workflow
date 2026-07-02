package mcp

// oauth.go — OAuth 2.0 Authorization Code flow with PKCE for remote MCP servers.
//
// The flow:
//  1. Generate a code verifier and S256 code challenge.
//  2. Start a local HTTP server on a random port to receive the callback.
//  3. Open the user's browser to the authorization URL.
//  4. Exchange the authorization code for tokens at the token endpoint.
//  5. Store tokens via CachedSaveToken.
//  6. Shut down the local server.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// OAuthTimeout is the maximum time the user has to complete the OAuth dance
// in the browser before the callback server gives up.
const OAuthTimeout = 5 * time.Minute

// pkceChallenge returns a random code verifier and its S256 challenge.
func pkceChallenge() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("pkce: cannot generate verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// openURL opens the given URL in the user's default browser.
func openURL(u string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{u}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", u}
	default:
		cmd = "xdg-open"
		args = []string{u}
	}
	return exec.Command(cmd, args...).Start()
}

// StartOAuthFlow performs the OAuth Authorization Code flow with PKCE for
// the given catalog entry. It opens the user's browser, starts a local
// callback server, exchanges the code for tokens, and persists them.
//
// The flow times out after OAuthTimeout.
func StartOAuthFlow(ctx context.Context, entry CatalogEntry) error {
	if !entry.HasOAuth() {
		return fmt.Errorf("entry %q does not require OAuth (AuthType=%q)", entry.ID, entry.AuthType)
	}
	if entry.ClientID == "" {
		return fmt.Errorf("entry %q has no ClientID configured", entry.ID)
	}
	if entry.AuthorizationURL == "" || entry.TokenURL == "" {
		return fmt.Errorf("entry %q has no AuthorizationURL or TokenURL", entry.ID)
	}

	// 1. PKCE setup.
	verifier, challenge, err := pkceChallenge()
	if err != nil {
		return fmt.Errorf("oauth: %w", err)
	}

	// 2. Find a random available port and start the callback listener.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("oauth: cannot start callback server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// 3. Build the authorization URL.
	authURL, err := url.Parse(entry.AuthorizationURL)
	if err != nil {
		listener.Close()
		return fmt.Errorf("oauth: invalid authorization URL: %w", err)
	}
	q := authURL.Query()
	q.Set("response_type", "code")
	q.Set("client_id", entry.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	if len(entry.Scopes) > 0 {
		q.Set("scope", strings.Join(entry.Scopes, " "))
	}
	// Add a random state to prevent CSRF on the callback.
	state, _, _ := pkceChallenge()
	q.Set("state", state)
	authURL.RawQuery = q.Encode()

	// 4. Start the callback HTTP server.
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		// Verify state.
		if r.FormValue("state") != state {
			errCh <- fmt.Errorf("oauth: state mismatch (possible CSRF)")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		// Check for error.
		if errStr := r.FormValue("error"); errStr != "" {
			errCh <- fmt.Errorf("oauth: authorization error: %s", errStr)
			http.Error(w, "Authorization error", http.StatusBadRequest)
			return
		}
		code := r.FormValue("code")
		if code == "" {
			errCh <- fmt.Errorf("oauth: no authorization code in callback")
			http.Error(w, "Missing code", http.StatusBadRequest)
			return
		}
		codeCh <- code
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body><p>OAuth complete — you can close this tab.</p></body></html>`)
	})}

	go server.Serve(listener) //nolint:errcheck

	// 5. Open browser.
	fmt.Printf("Opening browser for %s OAuth...\n", entry.Name)
	if err := openURL(authURL.String()); err != nil {
		server.Close()
		listener.Close()
		return fmt.Errorf("oauth: cannot open browser: %w", err)
	}

	// 6. Wait for the callback or timeout.
	oauthCtx, cancel := context.WithTimeout(ctx, OAuthTimeout)
	defer cancel()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		server.Close()
		listener.Close()
		return err
	case <-oauthCtx.Done():
		server.Close()
		listener.Close()
		return fmt.Errorf("oauth: timed out after %v waiting for browser callback", OAuthTimeout)
	}

	// 7. Exchange code for tokens.
	//   Graceful shutdown of the server (we no longer need it).
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx) //nolint:errcheck

	token, err := exchangeCode(ctx, entry, code, verifier, redirectURI)
	if err != nil {
		return fmt.Errorf("oauth: token exchange failed: %w", err)
	}

	// 8. Store the token.
	if err := CachedSaveToken(entry.ID, token); err != nil {
		return fmt.Errorf("oauth: cannot save token: %w", err)
	}

	fmt.Printf("OAuth complete for %s — token saved.\n", entry.Name)
	return nil
}

// exchangeCode POSTs the authorization code to the token endpoint and
// returns the parsed OAuthToken.
func exchangeCode(ctx context.Context, entry CatalogEntry, code, verifier, redirectURI string) (*OAuthToken, error) {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
		"client_id":    {entry.ClientID},
		"code_verifier": {verifier},
	}
	if entry.ClientSecret != "" {
		data.Set("client_secret", entry.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, entry.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("cannot create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read token response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response.
	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"`
		ExpiresIn    int    `json:"expires_in,omitempty"`
		TokenType    string `json:"token_type,omitempty"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("cannot parse token response: %w", err)
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token: %s", string(body))
	}

	tok := &OAuthToken{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		TokenType:    raw.TokenType,
	}
	if raw.ExpiresIn > 0 {
		tok.ExpiresAt = time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	}
	return tok, nil
}

// RefreshToken uses the refresh token (if available) to get a new access token.
// Returns the updated token on success.
func RefreshToken(ctx context.Context, entry CatalogEntry, tok *OAuthToken) (*OAuthToken, error) {
	if tok.RefreshToken == "" {
		return nil, fmt.Errorf("oauth: no refresh token available for %q", entry.ID)
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tok.RefreshToken},
		"client_id":     {entry.ClientID},
	}
	if entry.ClientSecret != "" {
		data.Set("client_secret", entry.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, entry.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("cannot create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read refresh response: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("refresh endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"`
		ExpiresIn    int    `json:"expires_in,omitempty"`
		TokenType    string `json:"token_type,omitempty"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("cannot parse refresh response: %w", err)
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("refresh response missing access_token")
	}

	newTok := &OAuthToken{
		AccessToken: raw.AccessToken,
		TokenType:   raw.TokenType,
	}
	// Use the new refresh token if provided, otherwise keep the old one.
	if raw.RefreshToken != "" {
		newTok.RefreshToken = raw.RefreshToken
	} else {
		newTok.RefreshToken = tok.RefreshToken
	}
	if raw.ExpiresIn > 0 {
		newTok.ExpiresAt = time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	}
	return newTok, nil
}

// EnsureValidToken returns a valid token for the given server, refreshing it
// if necessary. If no token exists, it starts the OAuth flow.
func EnsureValidToken(ctx context.Context, entry CatalogEntry) (*OAuthToken, error) {
	tok, err := CachedLoadToken(entry.ID)
	if err != nil {
		return nil, fmt.Errorf("cannot load token for %q: %w", entry.ID, err)
	}
	if tok != nil && !tok.IsExpired() {
		return tok, nil
	}
	if tok != nil && tok.RefreshToken != "" {
		// Try refreshing before re-authenticating.
		newTok, err := RefreshToken(ctx, entry, tok)
		if err == nil {
			if saveErr := CachedSaveToken(entry.ID, newTok); saveErr != nil {
				return nil, fmt.Errorf("oauth: token refreshed but save failed: %w", saveErr)
			}
			return newTok, nil
		}
		// Refresh failed — fall through to full re-auth.
	}
	// No valid token — start the full OAuth flow.
	if err := StartOAuthFlow(ctx, entry); err != nil {
		return nil, err
	}
	return CachedLoadToken(entry.ID)
}

// OAuthTokenEnv returns a map suitable for MergeEnv that injects the
// OAuth access token as an env var (Bearer token in the format the MCP
// server expects: "AUTHORIZATION" → "Bearer <token>").
func OAuthTokenEnv(entry CatalogEntry, tok *OAuthToken) map[string]string {
	if tok == nil || tok.AccessToken == "" {
		return nil
	}
	return map[string]string{
		"AUTHORIZATION": "Bearer " + tok.AccessToken,
	}
}
