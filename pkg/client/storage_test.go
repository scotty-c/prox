package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestGetStorageNames(t *testing.T) {
	var query string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Write([]byte(`{"data":[
			{"storage":"local-zfs","active":1},
			{"storage":"offline-nfs","active":0},
			{"storage":"ceph"}
		]}`))
	}))
	defer srv.Close()

	c, _ := NewClientWithTLS(srv.URL, "root@pam!t", "s", TLSOptions{Insecure: true})
	got, err := c.GetStorageNames(context.Background(), "pve", "rootdir")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"local-zfs", "ceph"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if query != "enabled=1&content=rootdir" {
		t.Fatalf("query = %q", query)
	}
}
