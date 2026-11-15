package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// taskLogServer serves total log lines the way Proxmox does: "limit" defaults
// to 50 when it is not sent
func taskLogServer(t *testing.T, total int) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/access/ticket" {
			_, _ = w.Write([]byte(`{"data":{"ticket":"t","CSRFPreventionToken":"c"}}`))
			return
		}
		start, limit := 0, 50
		if v := r.URL.Query().Get("start"); v != "" {
			start, _ = strconv.Atoi(v)
		}
		if v := r.URL.Query().Get("limit"); v != "" {
			limit, _ = strconv.Atoi(v)
		}
		out := "["
		for n := start; n < total && n < start+limit; n++ {
			if n > start {
				out += ","
			}
			out += fmt.Sprintf(`{"n":%d,"t":"line %d"}`, n+1, n+1)
		}
		_, _ = w.Write([]byte(`{"data":` + out + `]}`))
	}))
}

func TestGetTaskLogReturnsEveryLineWhenNoLimitIsGiven(t *testing.T) {
	for _, total := range []int{0, 7, 500, 501, 1200} {
		srv := taskLogServer(t, total)
		c, _ := NewClientWithTLS(srv.URL, "root@pam", "pw", TLSOptions{Insecure: true})

		lines, err := c.GetTaskLog(context.Background(), "pve1", "UPID:pve1:1:2:3:x:100:root@pam:", 0, 0)
		srv.Close()
		if err != nil {
			t.Fatalf("total %d: %v", total, err)
		}
		if len(lines) != total {
			t.Fatalf("total %d: got %d lines", total, len(lines))
		}
		if total > 0 && lines[total-1] != fmt.Sprintf("line %d", total) {
			t.Fatalf("total %d: last line = %q", total, lines[total-1])
		}
	}
}

func TestGetTaskLogFromOffsetReturnsTheRest(t *testing.T) {
	srv := taskLogServer(t, 130)
	defer srv.Close()
	c, _ := NewClientWithTLS(srv.URL, "root@pam", "pw", TLSOptions{Insecure: true})

	lines, err := c.GetTaskLog(context.Background(), "pve1", "UPID:pve1:1:2:3:x:100:root@pam:", 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 30 || lines[0] != "line 101" {
		t.Fatalf("got %d lines starting %q", len(lines), lines[0])
	}
}

func TestGetTaskLogHonoursAnExplicitLimit(t *testing.T) {
	srv := taskLogServer(t, 130)
	defer srv.Close()
	c, _ := NewClientWithTLS(srv.URL, "root@pam", "pw", TLSOptions{Insecure: true})

	lines, err := c.GetTaskLog(context.Background(), "pve1", "UPID:pve1:1:2:3:x:100:root@pam:", 0, 20)
	if err != nil || len(lines) != 20 {
		t.Fatalf("got %d lines, %v; want exactly 20", len(lines), err)
	}
}
