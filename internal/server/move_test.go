package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-home/internal/config"
)

// moveLinks starts Home behind a fake Core whose pcx_<who>_<export|import|read>
// tokens are Pocket Move links acting for alice@ or bob@.
func moveLinks(t *testing.T) *app {
	t.Helper()
	coreSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/auth/introspect" {
			io.WriteString(w, `{}`)
			return
		}
		var in struct {
			Token string `json:"token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		parts := strings.SplitN(strings.TrimPrefix(in.Token, "pcx_"), "_", 2)
		if len(parts) != 2 {
			io.WriteString(w, `{"active":false}`)
			return
		}
		scope := map[string]string{"export": core.ScopeMoveExport, "import": core.ScopeMoveImport, "read": "home:read"}[parts[1]]
		email := parts[0] + "@example.com"
		fmt.Fprintf(w, `{"active":true,"kind":"delegated","scope":%q,"audience":"home","user":{"id":1,"sub":%q,"email":%q,"name":%q,"role":"member"},"app":{"id":7,"slug":"move","name":"Pocket Move"}}`, scope, email, email, parts[0])
	}))
	t.Cleanup(coreSrv.Close)
	return newApp(t, func(c *config.Config) { c.CoreURL = coreSrv.URL })
}

func (a *app) raw(tok, method, path string, body []byte) (int, []byte) {
	a.t.Helper()
	req, _ := http.NewRequest(method, a.url+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

func TestMoveLinkScopes(t *testing.T) {
	a := moveLinks(t)
	exp, imp, read := "pcx_bob_export", "pcx_bob_import", "pcx_bob_read"
	for _, c := range []struct {
		tok, method, path string
		want              int
	}{
		{exp, "GET", core.PortablePath, 200},
		{exp, "GET", "/api/v1/export.json", 200},
		{exp, "GET", "/api/v1/export.csv", 200},
		{exp, "POST", "/api/v1/import", 403},
		{exp, "GET", "/api/v1/settings", 403},
		{exp, "GET", "/api/v1/homes", 403},
		{exp, "GET", "/api/v1/backup", 403},
		{imp, "GET", core.PortablePath, 200},
		{imp, "GET", "/api/v1/export.json", 403},
		{imp, "PUT", "/api/v1/settings", 403},
		{imp, "POST", "/api/v1/homes", 403},
		{imp, "POST", "/api/v1/records", 403},
		{read, "GET", core.PortablePath, 403},
		{read, "GET", "/api/v1/export.json", 403},
	} {
		if code, body := a.raw(c.tok, c.method, c.path, []byte("{}")); code != c.want {
			t.Errorf("%s %s %s → %d, want %d (%s)", c.tok, c.method, c.path, code, c.want, body)
		}
	}
	_, body := a.raw(exp, "GET", core.PortablePath, nil)
	var p core.Portable
	if err := json.Unmarshal(body, &p); err != nil || len(p.Exports) != 2 || len(p.Imports) != 1 || p.Exports[0].Kind != core.KindPocket {
		t.Fatalf("portable: %s", body)
	}
}

// Move brings Alice's home to Bob: the import adds once, and nothing of one
// person shows up for the other.
func TestMoveImportsAddOnce(t *testing.T) {
	a := moveLinks(t)
	a.seed(a.alice)
	code, export := a.raw("pcx_alice_export", "GET", "/api/v1/export.json", nil)
	if code != 200 || !strings.Contains(string(export), "Gas boiler") {
		t.Fatalf("alice's export: %d %s", code, export)
	}
	if _, bob := a.raw("pcx_bob_export", "GET", "/api/v1/export.json", nil); strings.Contains(string(bob), "Gas boiler") {
		t.Fatalf("bob's export holds alice's home: %s", bob)
	}
	imp := func() map[string]any {
		code, out := a.raw("pcx_bob_import", "POST", "/api/v1/import", export)
		if code != 200 {
			t.Fatalf("import → %d %s", code, out)
		}
		var m map[string]any
		_ = json.Unmarshal(out, &m)
		return m
	}
	if first := imp(); num(first, "homes") != 1 || num(first, "things") != 1 || num(first, "jobs") != 1 || num(first, "records") != 1 {
		t.Fatalf("first import: %v", first)
	}
	second := imp()
	for k, v := range second {
		if k != "skipped" && v.(float64) != 0 {
			t.Fatalf("second import added %s: %v", k, second)
		}
	}
	if n := len(a.list(a.bob, "/api/v1/homes")); n != 1 {
		t.Fatalf("bob's homes: %d", n)
	}
	if n := len(a.list(a.alice, "/api/v1/homes")); n != 1 {
		t.Fatalf("alice's homes: %d", n)
	}
}
