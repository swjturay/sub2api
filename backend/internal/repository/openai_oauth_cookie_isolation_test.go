package repository

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAITokenClientDoesNotReplayCookiesAcrossAccounts(t *testing.T) {
	cookies := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookies <- r.Header.Get("Cookie")
		http.SetCookie(w, &http.Cookie{Name: "oauth_session", Value: r.Header.Get("Authorization"), Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	first, err := createOpenAIReqClient("")
	require.NoError(t, err)
	second, err := createOpenAIReqClient("")
	require.NoError(t, err)
	require.Same(t, first, second, "connection pooling should remain enabled")
	for _, account := range []string{"account-one", "account-two"} {
		_, err = second.R().SetHeader("Authorization", account).Get(server.URL)
		require.NoError(t, err)
		require.Empty(t, <-cookies, "token requests must never replay another account's cookies")
	}

	defaultClient, err := getSharedReqClient(reqClientOptions{Timeout: 120 * time.Second})
	require.NoError(t, err)
	require.NotSame(t, first, defaultClient, "cookie policy must separate pooled clients")
}
