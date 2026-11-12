package client

import (
	"context"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/scotty-c/prox/pkg/config"
)

func newAuthServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"ticket":"t","CSRFPreventionToken":"c","username":"root@pam"}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestTLSVerificationDefaultsOn(t *testing.T) {
	srv := newAuthServer(t)

	c, err := NewClientWithTLS(srv.URL, "root@pam", "pw", TLSOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = c.Authenticate(context.Background())
	if !errors.Is(err, ErrCertificate) {
		t.Fatalf("expected certificate error for self-signed server, got %v", err)
	}
}

func TestTLSInsecure(t *testing.T) {
	srv := newAuthServer(t)
	c, err := NewClientWithTLS(srv.URL, "root@pam", "pw", TLSOptions{Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Authenticate(context.Background()); err != nil {
		t.Fatalf("insecure client should connect: %v", err)
	}
}

func TestTLSCustomCA(t *testing.T) {
	srv := newAuthServer(t)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(caFile, pemBytes, 0600); err != nil {
		t.Fatal(err)
	}

	c, err := NewClientWithTLS(srv.URL, "root@pam", "pw", TLSOptions{CACert: caFile})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Authenticate(context.Background()); err != nil {
		t.Fatalf("client trusting the server CA should connect: %v", err)
	}

	if _, err := NewClientWithTLS(srv.URL, "u", "p", TLSOptions{CACert: filepath.Join(t.TempDir(), "missing.pem")}); err == nil {
		t.Fatal("expected error for missing CA file")
	}
}

func TestResolveTLSOptionsPrecedence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PROX_INSECURE", "")
	t.Setenv("PROX_CA_CERT", "")
	t.Cleanup(func() { SetTLSFlagOverrides(false, false, "") })

	if err := config.CreateProfile("default", "u", "p", "https://pve:8006"); err != nil {
		t.Fatal(err)
	}
	if err := config.SetTLSOptions("default", TLSOptions{Insecure: true, CACert: "/profile.pem"}); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveTLSOptions()
	if err != nil || !got.Insecure || got.CACert != "/profile.pem" {
		t.Fatalf("profile: got %+v, %v", got, err)
	}

	t.Setenv("PROX_INSECURE", "false")
	t.Setenv("PROX_CA_CERT", "/env.pem")
	got, _ = ResolveTLSOptions()
	if got.Insecure || got.CACert != "/env.pem" {
		t.Fatalf("env should override profile: got %+v", got)
	}

	SetTLSFlagOverrides(true, true, "/flag.pem")
	got, _ = ResolveTLSOptions()
	if !got.Insecure || got.CACert != "/flag.pem" {
		t.Fatalf("flags should override env: got %+v", got)
	}

	t.Setenv("PROX_INSECURE", "maybe")
	if _, err := ResolveTLSOptions(); err == nil {
		t.Fatal("expected error for invalid PROX_INSECURE")
	}
}
