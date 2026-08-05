package oauth

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// resp builds a minimal *http.Response with the given status + JSON body.
func resp(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

// TestDecodeProtocol pins the OAuth/OIDC protocol face (/oauth/*): RFC 6749 /
// RFC 6750 shapes, and the classification callers act on — refresh-dead means
// clear the session and re-login, transient means keep it and retry.
func TestDecodeProtocol(t *testing.T) {
	t.Run("success returns the whole body", func(t *testing.T) {
		const body = `{"access_token":"tok","token_type":"Bearer","expires_in":900}`
		data, err := decodeProtocol(resp(200, body))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != body {
			t.Fatalf("got data %s", data)
		}
	})

	// RFC 6750 has no error code meaning "banned", so the OAuth server signals
	// it with a bare 403. Keying on the status is what keeps the distinct banned
	// page instead of degrading it to a re-login loop that hits the same ban.
	t.Run("403 is a ban", func(t *testing.T) {
		_, err := decodeProtocol(resp(403, `{"error":"invalid_token","error_description":"account banned"}`))
		if !IsBanned(err) {
			t.Fatalf("expected IsBanned, got %v", err)
		}
	})

	// Without the invalid_token mapping this lands on code 0, which IsTransient
	// reads as retryable — kungal would keep a dead session alive forever.
	t.Run("invalid_token → refresh dead, not transient", func(t *testing.T) {
		_, err := decodeProtocol(resp(401, `{"error":"invalid_token","error_description":"token expired"}`))
		if !IsRefreshTokenDead(err) {
			t.Fatalf("expected IsRefreshTokenDead, got %v", err)
		}
		if IsTransient(err) {
			t.Fatalf("invalid_token must not be transient")
		}
	})

	t.Run("invalid_grant → refresh dead", func(t *testing.T) {
		_, err := decodeProtocol(resp(400, `{"error":"invalid_grant","error_description":"refresh token revoked"}`))
		if !IsRefreshTokenDead(err) {
			t.Fatalf("expected IsRefreshTokenDead, got %v", err)
		}
	})

	t.Run("unknown error stays transient", func(t *testing.T) {
		_, err := decodeProtocol(resp(400, `{"error":"invalid_request","error_description":"bad"}`))
		if IsRefreshTokenDead(err) {
			t.Fatalf("invalid_request must not be refresh-dead")
		}
		if !IsTransient(err) {
			t.Fatalf("expected IsTransient for an unknown error")
		}
	})

	// server_error is the OP admitting an internal fault. Reading a 5xx as a
	// verdict on the credential would log the whole userbase out over a blip.
	t.Run("server_error 5xx stays transient", func(t *testing.T) {
		_, err := decodeProtocol(resp(500, `{"error":"server_error","error_description":"操作失败"}`))
		if !IsTransient(err) {
			t.Fatalf("expected IsTransient for 500")
		}
		if IsRefreshTokenDead(err) {
			t.Fatalf("a 5xx must never read as refresh-dead")
		}
	})
}

// TestDecodeHouse pins the house face (/auth/me, /auth/me/avatar), which keeps
// the {code,message,data} envelope permanently. Routing a house reply through
// decodeProtocol would hand the caller the whole envelope instead of its data,
// so the two readers must stay distinct.
func TestDecodeHouse(t *testing.T) {
	t.Run("envelope success returns inner data", func(t *testing.T) {
		data, err := decodeHouse(resp(200, `{"code":0,"message":"成功","data":{"name":"kun"}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != `{"name":"kun"}` {
			t.Fatalf("got data %s", data)
		}
	})

	t.Run("enveloped ban is a ban", func(t *testing.T) {
		_, err := decodeHouse(resp(403, `{"code":10014,"message":"账号已封禁"}`))
		if !IsBanned(err) {
			t.Fatalf("expected IsBanned, got %v", err)
		}
	})

	t.Run("non-zero code is an error", func(t *testing.T) {
		_, err := decodeHouse(resp(200, `{"code":10002,"message":"无效的令牌"}`))
		if !IsRefreshTokenDead(err) {
			t.Fatalf("expected IsRefreshTokenDead, got %v", err)
		}
	})
}
