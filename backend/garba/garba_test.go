package garba

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/xuri/excelize/v2"

	_ "github.com/arnav127/iima-garba/backend/migrations"
)

func setup(t *testing.T) (*Service, core.App) {
	t.Helper()
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.ResetBootstrapState() })
	return New(app), app
}

func signIn(t *testing.T, s *Service, email, name string) *core.Record {
	t.Helper()
	u, err := s.SignInUser(email, name)
	if err != nil {
		t.Fatalf("sign in %s: %v", email, err)
	}
	return u
}

func me(t *testing.T, s *Service, u *core.Record) MeResponse {
	t.Helper()
	m, err := s.Me(u)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestIIMASignInAndCohorts(t *testing.T) {
	s, _ := setup(t)
	cases := []struct {
		email, group string
		limit        int
		tone         string
	}{
		{"p26diya@iima.ac.in", "pgp1", 0, "student"},
		{"F26Arjun@IIMA.ac.in", "pgp1", 0, "student"},
		{"p25aarav@iima.ac.in", "student", 3, "student"},
		{"phd23neha@iima.ac.in", "student", 3, "student"},
		{"meena.k@iima.ac.in", "faculty", 3, "faculty"},
	}
	for _, c := range cases {
		u := signIn(t, s, c.email, "Some One")
		m := me(t, s, u)
		if m.User.Group != c.group || m.Limit != c.limit || m.Pass == nil || m.Pass.Kind != "own" || m.Pass.Tone != c.tone || m.Pass.Key == "" {
			t.Fatalf("%s: group=%s limit=%d pass=%+v", c.email, m.User.Group, m.Limit, m.Pass)
		}
	}
	// Signing in again doesn't create a second pass.
	u := signIn(t, s, "p25aarav@iima.ac.in", "Aarav Shah")
	if n, _ := s.app.CountRecords("passes", nil); n != 5 {
		t.Fatalf("passes: %d", n)
	}
	if _, err := s.SignInUser("stranger@gmail.com", "X"); err != ErrNotOnList {
		t.Fatalf("stranger: %v", err)
	}
	_ = u
}

func TestGuestsLimitsAndSignIn(t *testing.T) {
	s, _ := setup(t)
	aarav := signIn(t, s, "p25aarav@iima.ac.in", "Aarav Shah")
	diya := signIn(t, s, "p26diya@iima.ac.in", "Diya Rao")

	if _, err := s.AddGuest(diya, "Friend", ""); err == nil || !strings.Contains(err.Error(), "yourself only") {
		t.Fatalf("pgp1 should not add guests: %v", err)
	}
	m, err := s.AddGuest(aarav, "Riya  Patel", "Riya.Patel@gmail.com")
	if err != nil || m.Remaining != 2 || m.Guests[0].HolderName != "Riya Patel" || m.Guests[0].Key == "" || m.Guests[0].Tone != "guest" {
		t.Fatalf("add: %+v %v", m, err)
	}
	if _, err := s.AddGuest(aarav, "Riya again", "riya.patel@gmail.com"); err == nil {
		t.Fatal("duplicate email should fail")
	}
	if _, err := s.AddGuest(aarav, "Classmate", "p25x@iima.ac.in"); err == nil {
		t.Fatal("IIMA email as guest should fail")
	}
	s.AddGuest(aarav, "Kabir Desai", "")
	s.AddGuest(aarav, "Sunita Shah", "")
	if _, err := s.AddGuest(aarav, "One More", ""); err == nil {
		t.Fatal("limit should apply")
	}

	// Admin raises Aarav's limit only.
	lim := 5
	if p, err := s.UpdatePerson(aarav.Id, PersonUpdate{Limit: &lim}); err != nil || p.Limit != 5 {
		t.Fatalf("override: %+v %v", p, err)
	}
	if _, err := s.AddGuest(aarav, "One More", ""); err != nil {
		t.Fatal(err)
	}
	// And gives Diya (PGP1) one guest.
	one := 1
	s.UpdatePerson(diya.Id, PersonUpdate{Limit: &one})
	if _, err := s.AddGuest(diya, "Diya's Mom", ""); err != nil {
		t.Fatal(err)
	}

	// Riya can now sign in with Google and sees only her own pass.
	riya := signIn(t, s, "riya.patel@gmail.com", "Riya P")
	rm := me(t, s, riya)
	if rm.User.Group != "guest" || rm.Pass == nil || rm.Pass.HolderName != "Riya Patel" || rm.Limit != 0 || len(rm.Guests) != 0 {
		t.Fatalf("riya: %+v", rm)
	}
	if _, err := s.AddGuest(riya, "Plus one", ""); err == nil {
		t.Fatal("guests can't add guests")
	}

	// Removing frees the slot; the link stops working.
	link, _ := s.GuestLink(aarav, rm.Pass.ID, "https://garba.test")
	token := link[strings.LastIndex(link, "/")+1:]
	if lr, err := s.PassByLink(token); err != nil || lr.Pass.Key == "" {
		t.Fatalf("link: %v", err)
	}
	after, err := s.RemoveGuest(aarav, rm.Pass.ID)
	if err != nil || len(after.Guests) != 3 || after.Remaining != 2 {
		t.Fatalf("remove: %+v %v", after, err)
	}
	if _, err := s.PassByLink(token); err == nil {
		t.Fatal("removed pass link should fail")
	}
}

func TestRotatingQRScan(t *testing.T) {
	s, _ := setup(t)
	now := time.Date(2026, 10, 17, 19, 41, 5, 0, ist)
	s.Now = func() time.Time { return now }
	s.SaveSettings(SettingsInput{Event: EventInfo{Gates: 4}})
	aarav := signIn(t, s, "p25aarav@iima.ac.in", "Aarav Shah")
	vol := signIn(t, s, "p24meera@iima.ac.in", "Meera")
	m := me(t, s, aarav)
	p := m.Pass

	if r, _ := s.Scan(vol, QRPayload(p.Key, p.ID, now.Add(-2*time.Minute)), 3); r.Outcome != "expired" {
		t.Fatalf("old screenshot: %+v", r)
	}
	bad := QRPayload("wrong-key", p.ID, now)
	if r, _ := s.Scan(vol, bad, 3); r.Outcome != "invalid" {
		t.Fatalf("forged: %+v", r)
	}
	if r, _ := s.Scan(vol, "https://example.com", 3); r.Outcome != "invalid" {
		t.Fatalf("junk: %+v", r)
	}
	r, _ := s.Scan(vol, QRPayload(p.Key, p.ID, now.Add(-20*time.Second)), 3)
	if r.Outcome != "allowed" || r.Name != "Aarav Shah" || r.Tone != "student" || r.Entered != 1 {
		t.Fatalf("allowed: %+v", r)
	}
	r, _ = s.Scan(vol, QRPayload(p.Key, p.ID, now), 1)
	if r.Outcome != "used" || r.Sub != "Scanned at 7:41 PM · Gate 3" {
		t.Fatalf("used: %+v", r)
	}
	if m := me(t, s, aarav); m.Pass.EnteredAt == nil || *m.Pass.EnteredGate != 3 {
		t.Fatalf("pass should show entered: %+v", m.Pass)
	}

	// Typed code: needs an ID check, then admit.
	g, _ := s.AddGuest(aarav, "Kabir Desai", "")
	code := g.Guests[0].Code
	r, _ = s.Scan(vol, strings.ToLower(code), 2)
	if r.Outcome != "check" || r.PassID == "" || r.Tone != "guest" {
		t.Fatalf("manual: %+v", r)
	}
	// A typo (one character changed) is caught by the check character, before any lookup.
	typo := []byte(code)
	typo[0] = codeAlphabet[(codeIndex(typo[0])+1)%len(codeAlphabet)]
	if r2, _ := s.Scan(vol, string(typo), 2); r2.Outcome != "invalid" || !strings.Contains(r2.Title, "check out") {
		t.Fatalf("typo: %+v", r2)
	}
	if r, _ = s.Admit(vol, r.PassID, 2); r.Outcome != "allowed" {
		t.Fatalf("admit: %+v", r)
	}
	if _, err := s.RemoveGuest(aarav, g.Guests[0].ID); err == nil {
		t.Fatal("entered guest can't be removed")
	}
	stats, _ := s.Stats()
	if stats.Entered != 2 || stats.Members != 2 || stats.Guests != 1 {
		t.Fatalf("stats: %+v", stats)
	}
}

func TestPartialSettings(t *testing.T) {
	s, _ := setup(t)
	two := 2
	in := SettingsInput{PGP1Prefixes: []string{"P27", " f27 ", "bad prefix!"}}
	in.Limits.Faculty = &two
	out, err := s.SaveSettings(in)
	if err != nil || out.Limits != (Limits{PGP1: 0, Student: 3, Faculty: 2}) || strings.Join(out.PGP1Prefixes, ",") != "p27,f27" || out.Event.Gates != 2 {
		t.Fatalf("settings: %+v %v", out, err)
	}
	if u := signIn(t, s, "p27new@iima.ac.in", "New"); group(u) != "pgp1" {
		t.Fatalf("prefix change: %s", group(u))
	}
}

func TestConcurrentScansAdmitOnce(t *testing.T) {
	s, _ := setup(t)
	aarav := signIn(t, s, "p25aarav@iima.ac.in", "Aarav Shah")
	vol := signIn(t, s, "p24meera@iima.ac.in", "Meera")
	p := me(t, s, aarav).Pass
	payload := QRPayload(p.Key, p.ID, time.Now())
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(gate int) {
			defer wg.Done()
			if r, err := s.Scan(vol, payload, gate); err == nil && r.Outcome == "allowed" {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}(i%3 + 1)
	}
	wg.Wait()
	if allowed != 1 {
		t.Fatalf("allowed %d times", allowed)
	}
}

func TestExchangeImport(t *testing.T) {
	s, _ := setup(t)
	f := excelize.NewFile()
	rows := [][]any{
		{"Full Name", "Email ID", "Institute", "Mobile"},
		{"Ishaan Mehta", "Ishaan.M@spjimr.org", "SPJIMR Mumbai", "9820000000"},
		{"Tara Singh", "", "XLRI Jamshedpur", ""},
		{"Bad Row", "not-an-email", "X", ""},
		{"Sneaky", "p25x@iima.ac.in", "IIMA", ""},
	}
	for i, r := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		f.SetSheetRow("Sheet1", cell, &r)
	}
	var buf bytes.Buffer
	f.Write(&buf)
	res, err := s.ImportExchange("guests.xlsx", buf.Bytes())
	if err != nil || res.Created != 2 || len(res.Errors) != 2 {
		t.Fatalf("xlsx: %+v %v", res, err)
	}
	// Re-upload as CSV updates instead of duplicating.
	res, err = s.ImportExchange("guests.csv", []byte("Name,Email,College\nIshaan Mehta,ishaan.m@spjimr.org,SPJIMR\n"))
	if err != nil || res.Updated != 1 || res.Created != 0 {
		t.Fatalf("csv: %+v %v", res, err)
	}
	ishaan := signIn(t, s, "ishaan.m@spjimr.org", "Ishaan")
	m := me(t, s, ishaan)
	if m.User.Group != "exchange" || m.Pass == nil || m.Pass.Tone != "exchange" || !validCode(m.Pass.Code) || *m.Pass.College != "SPJIMR" {
		t.Fatalf("exchange me: %+v", m.Pass)
	}
	links, err := s.ExchangeLinks("https://garba.test")
	if err != nil || !strings.Contains(string(links), "https://garba.test/p/") || !strings.Contains(string(links), "Tara Singh") {
		t.Fatalf("links: %s %v", links, err)
	}
}

func validCode(c string) bool {
	n, ok := normalizePassCode(c)
	return ok && n == c && len(c) == 8 && c[4] == '-'
}

func TestPassCodes(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		c := newPassCode()
		if !validCode(c) || seen[c] {
			t.Fatalf("bad or duplicate code %q", c)
		}
		seen[c] = true
		raw := strings.ReplaceAll(c, "-", "")

		// Messy typing is fine.
		if n, ok := normalizePassCode(" " + strings.ToLower(raw[:3]) + " " + raw[3:] + " "); !ok || n != c {
			t.Fatalf("normalize %q: %q %v", c, n, ok)
		}
		// Every single-character substitution is caught.
		for pos := 0; pos < len(raw); pos++ {
			for _, alt := range []byte(codeAlphabet) {
				if alt == raw[pos] {
					continue
				}
				b := []byte(raw)
				b[pos] = alt
				if _, ok := normalizePassCode(string(b)); ok {
					t.Fatalf("substitution %q -> %q not caught", raw, b)
				}
			}
		}
	}
	// Look-alike characters are never valid.
	for _, bad := range []string{"0000-000", "OOOO-OOO", "1ILU-234", "GRB-0417", "ABC", ""} {
		if _, ok := normalizePassCode(bad); ok {
			t.Fatalf("%q accepted", bad)
		}
	}
	// Adjacent swaps are caught most of the time (Luhn mod N misses only a few pairs).
	caught, total := 0, 0
	for c := range seen {
		raw := []byte(strings.ReplaceAll(c, "-", ""))
		for i := 0; i+1 < len(raw); i++ {
			if raw[i] == raw[i+1] {
				continue
			}
			b := append([]byte(nil), raw...)
			b[i], b[i+1] = b[i+1], b[i]
			total++
			if _, ok := normalizePassCode(string(b)); !ok {
				caught++
			}
		}
	}
	if float64(caught)/float64(total) < 0.9 {
		t.Fatalf("only %d/%d swaps caught", caught, total)
	}
	t.Logf("adjacent swaps caught: %d/%d", caught, total)
}
