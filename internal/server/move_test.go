package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abjelosevic88/pocket-homelab-booking/internal/config"
	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
)

// moveLinks starts Booking behind a fake Core whose pcx_<who>_<export|import|read>
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
		scope := map[string]string{"export": core.ScopeMoveExport, "import": core.ScopeMoveImport, "read": "booking:read"}[parts[1]]
		email := parts[0] + "@example.com"
		fmt.Fprintf(w, `{"active":true,"kind":"delegated","scope":%q,"audience":"booking","user":{"id":1,"sub":%q,"email":%q,"name":%q,"role":"member"},"app":{"id":7,"slug":"move","name":"Pocket Move"}}`, scope, email, email, parts[0])
	}))
	t.Cleanup(coreSrv.Close)
	return newApp(t, func(c *config.Config) { c.CoreURL = coreSrv.URL })
}

func (a *app) raw(tok, method, path, ctype string, body []byte) (int, []byte) {
	a.t.Helper()
	req, _ := http.NewRequest(method, a.url+path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

func (a *app) moveImport(tok, path string, data []byte) map[string]any {
	a.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "upload")
	_, _ = fw.Write(data)
	_ = mw.Close()
	code, out := a.raw(tok, "POST", path, mw.FormDataContentType(), buf.Bytes())
	if code != 200 {
		a.t.Fatalf("POST %s → %d %s", path, code, out)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	return m
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
		{exp, "POST", "/api/v1/import", 403},
		{exp, "GET", "/api/v1/settings", 403},
		{exp, "GET", "/api/v1/backup", 403},
		{exp, "GET", "/api/v1/users", 403},
		{imp, "GET", core.PortablePath, 200},
		{imp, "GET", "/api/v1/export.json", 403},
		{imp, "PUT", "/api/v1/settings", 403},
		{imp, "POST", "/api/v1/types", 403},
		{imp, "POST", "/api/v1/schedules", 403},
		{read, "GET", core.PortablePath, 403},
		{read, "GET", "/api/v1/export.json", 403},
		{read, "POST", "/api/v1/import", 403},
	} {
		if code, body := a.raw(c.tok, c.method, c.path, "application/json", []byte("{}")); code != c.want {
			t.Errorf("%s %s %s → %d, want %d (%s)", c.tok, c.method, c.path, code, c.want, body)
		}
	}
	_, body := a.raw(exp, "GET", core.PortablePath, "", nil)
	var p core.Portable
	if err := json.Unmarshal(body, &p); err != nil || len(p.Exports) != 1 || len(p.Imports) != 2 || p.Exports[0].Kind != core.KindPocket {
		t.Fatalf("portable: %s", body)
	}
}

// Move brings Alice's pages and bookings to Bob: each import adds once, and
// nothing of one person shows up for the other.
func TestMoveImportsAddOnce(t *testing.T) {
	a := moveLinks(t)
	ty := alicePage(t, a, nil)
	start := firstSlot(t, a, a.alice, id(ty))
	a.ok(a.alice, "POST", "/api/v1/bookings", map[string]any{"type_id": n(id(ty)), "start": start, "name": "Ana", "email": "ana@example.com"})

	code, export := a.raw("pcx_alice_export", "GET", "/api/v1/export.json", "", nil)
	if code != 200 || !strings.Contains(string(export), "ana@example.com") || !strings.Contains(string(export), "Intro call") {
		t.Fatalf("alice's export: %d %s", code, export)
	}
	if _, bob := a.raw("pcx_bob_export", "GET", "/api/v1/export.json", "", nil); strings.Contains(string(bob), "ana@example.com") || strings.Contains(string(bob), "Intro call") {
		t.Fatalf("bob's export holds alice's bookings: %s", bob)
	}

	first := a.moveImport("pcx_bob_import", "/api/v1/import", export)
	if num(first, "types") != 1 || num(first, "bookings") != 1 || num(first, "imported") < 2 {
		t.Fatalf("first import: %v", first)
	}
	second := a.moveImport("pcx_bob_import", "/api/v1/import", export)
	if num(second, "imported") != 0 {
		t.Fatalf("second import added: %v", second)
	}

	csv := []byte("Invitee Name,Invitee Email,Event Type Name,Start Date & Time,End Date & Time,Canceled\n" +
		"Marko,marko@example.com,Consultation,2026-03-02 10:00,2026-03-02 10:30,false\n" +
		"Jana,jana@example.com,Consultation,2026-03-03 11:00,2026-03-03 11:30,false\n")
	c1 := a.moveImport("pcx_bob_import", "/api/v1/import", csv)
	c2 := a.moveImport("pcx_bob_import", "/api/v1/import", csv)
	if num(c1, "bookings") != 2 || num(c1, "types") != 1 || num(c2, "imported") != 0 {
		t.Fatalf("csv imports: %v then %v", c1, c2)
	}

	var bobN, aliceN int
	_ = a.st.DB.QueryRow(`SELECT COUNT(1) FROM bookings WHERE user_id = ?`, a.ids["Bob"]).Scan(&bobN)
	_ = a.st.DB.QueryRow(`SELECT COUNT(1) FROM bookings WHERE user_id = ?`, a.ids["Alice"]).Scan(&aliceN)
	if bobN != 3 || aliceN != 1 {
		t.Fatalf("bookings: bob %d, alice %d", bobN, aliceN)
	}
	if ts := a.ok(a.alice, "GET", "/api/v1/types", nil)["types"].([]any); len(ts) != 1 {
		t.Fatalf("alice's pages changed: %v", ts)
	}
}
