package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// countingServer serves an empty cluster and counts /cluster/resources requests
func countingServer(t *testing.T) (*ProxmoxClient, *int32) {
	t.Helper()
	var fetches int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/access/ticket"):
			w.Write([]byte(`{"data":{"ticket":"t","CSRFPreventionToken":"c"}}`))
		case strings.HasSuffix(r.URL.Path, "/cluster/resources"):
			atomic.AddInt32(&fetches, 1)
			w.Write([]byte(`{"data":[]}`)) // an empty cluster still counts as fetched
		}
	}))
	t.Cleanup(srv.Close)
	c, err := NewClientWithTLS(srv.URL, "root@pam", "pw", TLSOptions{Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	return c, &fetches
}

func TestClusterResourcesAreFetchedOnceForRepeatedLookups(t *testing.T) {
	c, fetches := countingServer(t)
	for i := 0; i < 5; i++ {
		if _, err := c.GetClusterResources(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if n := atomic.LoadInt32(fetches); n != 1 {
		t.Errorf("fetched %d times for 5 lookups, want 1", n)
	}
}

func TestClusterResourcesAreFetchedOnceForConcurrentLookups(t *testing.T) {
	c, fetches := countingServer(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.GetClusterResources(context.Background())
		}()
	}
	wg.Wait()
	if n := atomic.LoadInt32(fetches); n != 1 {
		t.Errorf("fetched %d times for 20 concurrent lookups, want 1", n)
	}
}

func TestClearingTheListingForcesAFreshFetch(t *testing.T) {
	c, fetches := countingServer(t)
	ctx := context.Background()
	_, _ = c.GetClusterResources(ctx)
	c.ClearClusterResourcesCache()
	_, _ = c.GetClusterResources(ctx)
	if n := atomic.LoadInt32(fetches); n != 2 {
		t.Errorf("fetched %d times, want 2 (before and after the clear)", n)
	}
}

func TestAFailedFetchIsNotRemembered(t *testing.T) {
	var calls int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/access/ticket") {
			w.Write([]byte(`{"data":{"ticket":"t","CSRFPreventionToken":"c"}}`))
			return
		}
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"data":[{"id":"qemu/100","type":"qemu","node":"pve1","vmid":100,"name":"web"}]}`))
	}))
	defer srv.Close()
	c, _ := NewClientWithTLS(srv.URL, "root@pam", "pw", TLSOptions{Insecure: true})

	if _, err := c.GetClusterResources(context.Background()); err == nil {
		t.Fatal("first fetch should fail")
	}
	got, err := c.GetClusterResources(context.Background())
	if err != nil || len(got) != 1 {
		t.Errorf("retry after a failure: %v, %v", got, err)
	}
}

func TestCreateClientReturnsTheSameClientEveryTime(t *testing.T) {
	t.Setenv("PROX_URL", "https://pve.example:8006")
	t.Setenv("PROX_USER", "root@pam")
	t.Setenv("PROX_PASSWORD", "pw")
	sharedClient = nil // other tests in this process may have made one
	t.Cleanup(func() { sharedClient = nil })

	first, err := CreateClient()
	if err != nil {
		t.Fatal(err)
	}
	// Even if the environment changed, a run keeps the client it started with
	t.Setenv("PROX_URL", "https://other.example:8006")
	second, err := CreateClient()
	if err != nil || first != second {
		t.Errorf("CreateClient returned a different client (%v)", err)
	}
}
