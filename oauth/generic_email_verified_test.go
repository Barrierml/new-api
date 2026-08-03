package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenericOAuthProviderReadsEmailVerifiedClaim(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sub":"google-user","email":"user@example.com","email_verified":true}`))
	}))
	t.Cleanup(server.Close)

	provider := NewGenericOAuthProvider(&model.CustomOAuthProvider{
		Slug: "google", UserInfoEndpoint: server.URL, UserIdField: "sub", EmailField: "email",
	})
	user, err := provider.GetUserInfo(context.Background(), &OAuthToken{AccessToken: "test-token", TokenType: "Bearer"})

	require.NoError(t, err)
	assert.Equal(t, "user@example.com", user.Email)
	assert.True(t, user.EmailVerified)
}

func TestGenericOAuthProviderDefaultsEmailVerifiedToFalse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sub":"provider-user","email":"user@example.com"}`))
	}))
	t.Cleanup(server.Close)

	provider := NewGenericOAuthProvider(&model.CustomOAuthProvider{
		Slug: "generic", UserInfoEndpoint: server.URL, UserIdField: "sub", EmailField: "email",
	})
	user, err := provider.GetUserInfo(context.Background(), &OAuthToken{AccessToken: "test-token"})

	require.NoError(t, err)
	assert.False(t, user.EmailVerified)
}
