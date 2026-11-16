package upgrade

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetCapsSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("0123456789"))
	}))
	defer srv.Close()

	orig := maxDownload
	defer func() { maxDownload = orig }()
	maxDownload = 10
	if b, err := get(t.Context(), srv.URL); err != nil || string(b) != "0123456789" {
		t.Errorf("at the cap: %q, %v", b, err)
	}
	maxDownload = 9
	if _, err := get(t.Context(), srv.URL); err == nil || !strings.HasSuffix(err.Error(), "it's larger than 0 MB") {
		t.Errorf("over the cap: %v", err)
	}
}
