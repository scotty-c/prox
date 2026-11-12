package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestReauthenticatesOnceWhenTicketExpires(t *testing.T) {
	var logins, versionCalls int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			n := atomic.AddInt32(&logins, 1)
			w.Write([]byte(`{"data":{"ticket":"ticket-` + string(rune('0'+n)) + `","CSRFPreventionToken":"c"}}`))
		case "/api2/json/version":
			atomic.AddInt32(&versionCalls, 1)
			// The first ticket has "expired"
			if strings.Contains(r.Header.Get("Cookie"), "ticket-1") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Write([]byte(`{"data":{"version":"8.2"}}`))
		}
	}))
	defer srv.Close()

	c, _ := NewClientWithTLS(srv.URL, "root@pam", "pw", TLSOptions{Insecure: true})
	v, err := c.GetVersion(context.Background())
	if err != nil || v.Version != "8.2" {
		t.Fatalf("GetVersion = %+v, %v", v, err)
	}
	if logins != 2 || versionCalls != 2 {
		t.Fatalf("logins = %d, version calls = %d; want 2 and 2", logins, versionCalls)
	}
}

func TestGivesUpAfterOneReauthentication(t *testing.T) {
	var logins int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/access/ticket" {
			atomic.AddInt32(&logins, 1)
			w.Write([]byte(`{"data":{"ticket":"t","CSRFPreventionToken":"c"}}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, _ := NewClientWithTLS(srv.URL, "root@pam", "pw", TLSOptions{Insecure: true})
	if _, err := c.GetVersion(context.Background()); err == nil || !strings.Contains(err.Error(), "status 401") {
		t.Fatalf("expected 401 error, got %v", err)
	}
	if logins != 2 {
		t.Fatalf("logins = %d, want 2", logins)
	}
}

func TestAPIErrorMessage(t *testing.T) {
	tests := []struct {
		reason, body, want string
	}{
		{"VM 100 not running", `{"data":null}`, "VM 100 not running"},
		{"Parameter verification failed.", `{"data":null,"errors":{"memory":"value must be at least 16","cores":"invalid"}}`,
			"Parameter verification failed. (cores: invalid; memory: value must be at least 16)"},
		{"", `{"data":null,"message":"permission denied - invalid PVE ticket\n"}`, "permission denied - invalid PVE ticket"},
		{"Bad Gateway", "<html>" + strings.Repeat("x", 400) + "</html>", "Bad Gateway <html>" + strings.Repeat("x", 294) + "..."},
		{"", "", "no details returned"},
	}
	for _, tt := range tests {
		if got := apiErrorMessage(tt.reason, []byte(tt.body)); got != tt.want {
			t.Errorf("apiErrorMessage(%q, %q)\n got: %q\nwant: %q", tt.reason, tt.body, got, tt.want)
		}
	}
}

func TestPathSegmentsAreEscaped(t *testing.T) {
	var gotPath string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"OK"}}`))
	}))
	defer srv.Close()

	c, _ := NewClientWithTLS(srv.URL, "root@pam!t", "s", TLSOptions{Insecure: true})
	upid := "UPID:pve:00001:00002:00003:qmstart:100:root@pam:"
	if _, err := c.GetTaskStatus(context.Background(), "pve", upid); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gotPath, "/api2/json/nodes/pve/tasks/UPID:pve:") || !strings.HasSuffix(gotPath, "/status") {
		t.Fatalf("unexpected path %q", gotPath)
	}
}
