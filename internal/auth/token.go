package auth

import (
	"context"
)

// ValidateCredentials attempts a token refresh to verify credentials are valid.
func ValidateCredentials(clientID, clientSecret, refreshToken string) error {
	auth := NewOAuthAuthenticator(clientID, clientSecret, refreshToken)
	_, err := auth.AccessToken(context.Background())
	return err
}
