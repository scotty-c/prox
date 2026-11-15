package client

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scotty-c/prox/pkg/apierr"
	"github.com/scotty-c/prox/pkg/config"
)

// tlsServer is a Proxmox-like server with a self-signed certificate; it
// answers the version request, which is all an API token login needs
func tlsServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"version":"8.2.4","release":"8.2"}}`)
	}))
	t.Cleanup(srv.Close)
	return srv, fingerprintOf(srv.Certificate().Raw)
}

const token = "ci@pve!deploy"

func TestACertificateNobodyTrustsIsRefused(t *testing.T) {
	srv, _ := tlsServer(t)
	c, err := NewClientWithTLS(srv.URL, token, "secret", TLSOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Authenticate(context.Background()); !errors.Is(err, apierr.ErrCertificate) {
		t.Fatalf("err = %v, want a certificate error", err)
	}
}

func TestAPinnedCertificateIsTrustedWhateverItsNameOrIssuer(t *testing.T) {
	srv, fp := tlsServer(t)
	c, err := NewClientWithTLS(srv.URL, token, "secret", TLSOptions{Fingerprint: fp})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Authenticate(context.Background()); err != nil {
		t.Fatalf("pinned certificate refused: %v", err)
	}
	v, err := c.GetVersion(context.Background())
	if err != nil || v.Version != "8.2.4" {
		t.Fatalf("GetVersion = %+v, %v", v, err)
	}
}

func TestAnyOtherCertificateIsRefusedAndBothFingerprintsAreShown(t *testing.T) {
	srv, fp := tlsServer(t)
	other := strings.Repeat("AB:", 31) + "AB"
	c, err := NewClientWithTLS(srv.URL, token, "secret", TLSOptions{Fingerprint: other})
	if err != nil {
		t.Fatal(err)
	}
	err = c.Authenticate(context.Background())
	if !errors.Is(err, apierr.ErrCertificate) {
		t.Fatalf("err = %v, want a certificate error", err)
	}
	var mismatch *FingerprintMismatchError
	if !errors.As(err, &mismatch) || mismatch.Got != fp || mismatch.Want != other {
		t.Fatalf("mismatch = %+v, want got %s want %s", mismatch, fp, other)
	}
	for _, want := range []string{fp, other, "prox config setup --fingerprint " + fp} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q:\n%v", want, err)
		}
	}
}

func TestInsecureStillOverridesAPin(t *testing.T) {
	srv, _ := tlsServer(t)
	c, _ := NewClientWithTLS(srv.URL, token, "secret", TLSOptions{Insecure: true, Fingerprint: strings.Repeat("AB:", 31) + "AB"})
	if err := c.Authenticate(context.Background()); err != nil {
		t.Fatalf("--insecure should win: %v", err)
	}
}

func TestProbeShowsTheCertificateWithoutTrustingIt(t *testing.T) {
	srv, fp := tlsServer(t)
	cert, err := ProbeCertificate(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(srv.Certificate().Raw)
	if cert.Fingerprint != fp || len(cert.Fingerprint) != len(sum)*3-1 {
		t.Errorf("fingerprint %q, want %q", cert.Fingerprint, fp)
	}
	if cert.Subject == "" || cert.NotAfter.IsZero() {
		t.Errorf("incomplete description: %+v", cert)
	}
	// the fingerprint is in the form the configuration accepts back
	if got, err := config.NormalizeFingerprint(cert.Fingerprint); err != nil || got != cert.Fingerprint {
		t.Errorf("NormalizeFingerprint(%q) = %q, %v", cert.Fingerprint, got, err)
	}
}

func TestProbeReportsAServerThatIsNotThere(t *testing.T) {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	if _, err := ProbeCertificate(context.Background(), url); err == nil {
		t.Fatal("want an error for a closed port")
	}
	if _, err := ProbeCertificate(context.Background(), "not a url"); err == nil {
		t.Fatal("want an error for a bad URL")
	}
}

func TestFingerprintSettingsReplaceTheCAFromLowerLayers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(func() { SetTLSFlagOverrides(false, false, "", "") })
	fp := strings.Repeat("CD:", 31) + "CD"
	if _, err := config.SaveProfile("default", "u", "p", "https://pve:8006", config.TLSOptions{CACert: "/profile.pem"}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PROX_FINGERPRINT", strings.ToLower(strings.ReplaceAll(fp, ":", "")))
	got, err := ResolveTLSOptions()
	if err != nil || got.Fingerprint != fp || got.CACert != "" {
		t.Fatalf("env fingerprint: %+v, %v (a fingerprint replaces the profile's CA)", got, err)
	}

	SetTLSFlagOverrides(false, false, "/flag.pem", "")
	got, _ = ResolveTLSOptions()
	if got.CACert != "/flag.pem" || got.Fingerprint != "" {
		t.Fatalf("--ca-cert should replace the pin from the environment: %+v", got)
	}

	t.Setenv("PROX_FINGERPRINT", "")
	SetTLSFlagOverrides(false, false, "", fp)
	got, _ = ResolveTLSOptions()
	if got.Fingerprint != fp || got.CACert != "" {
		t.Fatalf("--fingerprint should replace the profile's CA: %+v", got)
	}

	SetTLSFlagOverrides(false, false, "", "not-a-fingerprint")
	if _, err := ResolveTLSOptions(); err == nil || !strings.Contains(err.Error(), "--fingerprint") {
		t.Fatalf("want an error naming --fingerprint, got %v", err)
	}
	SetTLSFlagOverrides(false, false, "", "")
	t.Setenv("PROX_FINGERPRINT", "zz")
	if _, err := ResolveTLSOptions(); err == nil || !strings.Contains(err.Error(), "PROX_FINGERPRINT") {
		t.Fatalf("want an error naming PROX_FINGERPRINT, got %v", err)
	}
}
