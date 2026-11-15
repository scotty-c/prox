package cmd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scotty-c/prox/pkg/apierr"
	"github.com/scotty-c/prox/pkg/client"
	c "github.com/scotty-c/prox/pkg/config"
)

// fixture is a server with a certificate nobody trusts, a stand-in for
// checkConnection that behaves as Proxmox would, and the answers a person types
type fixture struct {
	url, fingerprint string
	calls            []c.TLSOptions
	failWith         error // what the check returns once the certificate is trusted or chosen
}

func newFixture(t *testing.T) *fixture {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	cert, err := client.ProbeCertificate(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{url: srv.URL, fingerprint: cert.Fingerprint}
}

// check refuses the certificate unless it is pinned to the right fingerprint
func (f *fixture) check(_ context.Context, _, _, _ string, opts c.TLSOptions) (string, error) {
	f.calls = append(f.calls, opts)
	if opts.Insecure || opts.Fingerprint == f.fingerprint {
		if f.failWith != nil {
			return "", f.failWith
		}
		return "8.2.4", nil
	}
	return "", fmt.Errorf("%w: unknown authority", apierr.ErrCertificate)
}

// verify runs verifyAndTrust with typed answers, returning what it wrote
func (f *fixture) verify(answers string, interactive, chosen bool, opts c.TLSOptions) (c.TLSOptions, string, string, error) {
	var out bytes.Buffer
	got, version, err := verifyAndTrust(context.Background(), f.check, bufio.NewReader(strings.NewReader(answers)), &out,
		interactive, "lab", f.url, "root@pam", "pw", opts, chosen)
	return got, version, out.String(), err
}

func TestTypingYesPinsTheCertificateAndChecksTheLoginWithIt(t *testing.T) {
	f := newFixture(t)
	opts, version, out, err := f.verify("yes\n", true, false, c.TLSOptions{CACert: "/old-ca.pem"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Fingerprint != f.fingerprint || opts.CACert != "" {
		t.Errorf("saved %+v, want the fingerprint pinned and the CA replaced", opts)
	}
	if version != "8.2.4" {
		t.Errorf("version %q", version)
	}
	if len(f.calls) != 2 || f.calls[0].Fingerprint != "" || f.calls[1].Fingerprint != f.fingerprint {
		t.Errorf("checks = %+v: want one without the pin, then one with it", f.calls)
	}
	for _, want := range []string{f.url, f.fingerprint, "SHA-256", "Proxmox web interface", "Type 'yes'"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q:\n%s", want, out)
		}
	}
}

func TestAnythingButTheWholeWordYesIsNo(t *testing.T) {
	for _, answer := range []string{"y\n", "Y\n", "no\n", "\n", "", "yes please\n", "ye\n"} {
		f := newFixture(t)
		opts, _, _, err := f.verify(answer, true, false, c.TLSOptions{})
		if err == nil || !errors.Is(err, apierr.ErrCertificate) {
			t.Errorf("answer %q: err = %v, want a certificate error", answer, err)
		}
		if opts.Fingerprint != "" || len(f.calls) != 1 {
			t.Errorf("answer %q: pinned %q after %d checks; nothing should have been trusted", answer, opts.Fingerprint, len(f.calls))
		}
	}
}

func TestWithoutATerminalTheFingerprintIsShownButNeverTrusted(t *testing.T) {
	f := newFixture(t)
	opts, _, out, err := f.verify("yes\n", false, false, c.TLSOptions{})
	if err == nil || !errors.Is(err, apierr.ErrCertificate) {
		t.Fatalf("err = %v, want a certificate error", err)
	}
	if opts.Fingerprint != "" || len(f.calls) != 1 {
		t.Errorf("a script must not trust a certificate by itself: %+v after %d checks", opts, len(f.calls))
	}
	if !strings.Contains(out, f.fingerprint) || !strings.Contains(err.Error(), "--fingerprint "+f.fingerprint) {
		t.Errorf("should show the fingerprint and how to pass it\nout: %s\nerr: %v", out, err)
	}
}

func TestAChosenWayToTrustTheServerIsNotSecondGuessed(t *testing.T) {
	f := newFixture(t)
	_, _, out, err := f.verify("yes\n", true, true, c.TLSOptions{CACert: "/ca.pem"})
	if !errors.Is(err, apierr.ErrCertificate) {
		t.Fatalf("err = %v", err)
	}
	if out != "" || len(f.calls) != 1 {
		t.Errorf("--ca-cert that does not match must fail, not offer a pin: out %q, %d checks", out, len(f.calls))
	}
}

func TestInsecureNeedsNoQuestions(t *testing.T) {
	f := newFixture(t)
	opts, version, out, err := f.verify("", false, true, c.TLSOptions{Insecure: true})
	if err != nil || version == "" || out != "" || !opts.Insecure {
		t.Fatalf("got %+v %q %q %v", opts, version, out, err)
	}
}

func TestAChangedCertificateIsOfferedWithTheOldPinShown(t *testing.T) {
	f := newFixture(t)
	old := strings.Repeat("AB:", 31) + "AB"
	opts, _, out, err := f.verify("yes\n", true, false, c.TLSOptions{Fingerprint: old})
	if err != nil || opts.Fingerprint != f.fingerprint {
		t.Fatalf("err = %v, pinned %q", err, opts.Fingerprint)
	}
	for _, want := range []string{old, "different certificate", "do not trust this one"} {
		if !strings.Contains(out, want) {
			t.Errorf("a changed certificate should be called out; missing %q:\n%s", want, out)
		}
	}
}

func TestAFailureAfterTrustingMeansNothingIsSaved(t *testing.T) {
	f := newFixture(t)
	f.failWith = fmt.Errorf("%w: bad password", apierr.ErrAuth)
	_, _, _, err := f.verify("yes\n", true, false, c.TLSOptions{})
	if !errors.Is(err, apierr.ErrAuth) || !strings.Contains(err.Error(), "nothing was saved") {
		t.Fatalf("err = %v", err)
	}
}

func TestOtherFailuresCanBeSavedAnywayOnATerminalOnly(t *testing.T) {
	down := fmt.Errorf("%w at https://pve", apierr.ErrUnreachable)
	fail := func(context.Context, string, string, string, c.TLSOptions) (string, error) { return "", down }
	run := func(answer string, interactive bool) (string, error) {
		var out bytes.Buffer
		_, _, err := verifyAndTrust(context.Background(), fail, bufio.NewReader(strings.NewReader(answer)), &out,
			interactive, "lab", "https://pve:8006", "u", "p", c.TLSOptions{}, false)
		return out.String(), err
	}

	if _, err := run("y\n", true); err != nil {
		t.Errorf("answering y should save anyway, got %v", err)
	}
	for _, tc := range []struct {
		answer      string
		interactive bool
	}{{"n\n", true}, {"\n", true}, {"y\n", false}} {
		out, err := run(tc.answer, tc.interactive)
		if !errors.Is(err, apierr.ErrUnreachable) || !strings.Contains(err.Error(), "--no-verify") || !strings.Contains(err.Error(), "nothing was saved") {
			t.Errorf("%+v: err = %v", tc, err)
		}
		if !tc.interactive && out != "" {
			t.Errorf("a script should not be asked anything: %q", out)
		}
	}
}

func TestAnUnreachableCertificateServerKeepsTheOriginalError(t *testing.T) {
	// the probe cannot connect either, so the original error is what the user sees
	certErr := fmt.Errorf("%w: unknown authority", apierr.ErrCertificate)
	check := func(context.Context, string, string, string, c.TLSOptions) (string, error) { return "", certErr }
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	var out bytes.Buffer
	_, _, err := verifyAndTrust(context.Background(), check, bufio.NewReader(strings.NewReader("yes\n")), &out,
		true, "lab", url, "u", "p", c.TLSOptions{}, false)
	if err != certErr || out.Len() != 0 {
		t.Fatalf("err = %v, out = %q", err, out.String())
	}
}
