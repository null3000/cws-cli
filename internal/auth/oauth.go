package auth

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const chromeWebStoreScope = "https://www.googleapis.com/auth/chromewebstore"

// Authenticator handles OAuth2 token management.
type Authenticator interface {
	AccessToken(ctx context.Context) (string, error)
}

// OAuthAuthenticator refreshes access tokens using OAuth2 credentials.
type OAuthAuthenticator struct {
	config *oauth2.Config
	token  *oauth2.Token
}

// NewOAuthAuthenticator creates an authenticator from client credentials and a refresh token.
func NewOAuthAuthenticator(clientID, clientSecret, refreshToken string) *OAuthAuthenticator {
	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{chromeWebStoreScope},
	}

	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	return &OAuthAuthenticator{
		config: cfg,
		token:  token,
	}
}

// AccessToken returns a valid access token, refreshing if necessary.
func (a *OAuthAuthenticator) AccessToken(ctx context.Context) (string, error) {
	src := a.config.TokenSource(ctx, a.token)
	token, err := src.Token()
	if err != nil {
		return "", fmt.Errorf("authentication failed: %w. Run 'cws init' to reconfigure credentials", err)
	}
	return token.AccessToken, nil
}
