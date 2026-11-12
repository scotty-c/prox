package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPITokenAuth(t *testing.T) {
	const tokenID = "automation@pve!ci"
	const secret = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	var ticketCalls int

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/access/ticket" {
			ticketCalls++
		}
		if r.Header.Get("Authorization") != "PVEAPIToken="+tokenID+"="+secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Cookie") != "" || r.Header.Get("CSRFPreventionToken") != "" {
			t.Errorf("token request should not send ticket headers")
		}
		w.Write([]byte(`{"data":{"version":"8.2.4","release":"8.2","repoid":"x"}}`))
	}))
	defer srv.Close()

	c, err := NewClientWithTLS(srv.URL, tokenID, secret, TLSOptions{Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate with token: %v", err)
	}
	v, err := c.GetVersion(context.Background())
	if err != nil || v.Version != "8.2.4" {
		t.Fatalf("GetVersion = %+v, %v", v, err)
	}
	if ticketCalls != 0 {
		t.Fatalf("token auth should never request a ticket, made %d calls", ticketCalls)
	}

	bad, _ := NewClientWithTLS(srv.URL, tokenID, "wrong", TLSOptions{Insecure: true})
	if err := bad.Authenticate(context.Background()); err == nil {
		t.Fatal("expected error for wrong token secret")
	}
}

func TestIsAPIToken(t *testing.T) {
	for user, want := range map[string]bool{"root@pam": false, "root@pam!prox": true, "ci@pve!deploy-1": true} {
		if got := IsAPIToken(user); got != want {
			t.Errorf("IsAPIToken(%q) = %v, want %v", user, got, want)
		}
	}
}
