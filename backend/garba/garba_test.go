package garba

import (
	"bytes"
	"fmt"
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
		{"bpgpbaai26rommeln@iima.ac.in", "student", 3, "student"},
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
	if n, _ := s.app.CountRecords("passes", nil); n != 6 {
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
	// Same volunteer re-scanning within 30 s (e.g. the reply was lost): still green.
	if r, _ := s.Scan(vol, QRPayload(p.Key, p.ID, now), 3); r.Outcome != "allowed" || !strings.HasPrefix(r.Sub, "You let them in 0s ago") {
		t.Fatalf("own rescan: %+v", r)
	}
	// Anyone else, or later: red, with when and where.
	other := signIn(t, s, "p24ravi@iima.ac.in", "Ravi")
	r, _ = s.Scan(other, QRPayload(p.Key, p.ID, now), 1)
	if r.Outcome != "used" || r.Sub != "Scanned at 7:41 PM · Gate 3" {
		t.Fatalf("used: %+v", r)
	}
	later := now.Add(31 * time.Second)
	s.Now = func() time.Time { return later }
	if r, _ := s.Scan(vol, QRPayload(p.Key, p.ID, later), 3); r.Outcome != "used" {
		t.Fatalf("own rescan after grace: %+v", r)
	}
	s.Now = func() time.Time { return now }
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
	// Reply lost, volunteer types the code again: green, not "Already used".
	if r2, _ := s.Scan(vol, code, 2); r2.Outcome != "allowed" || !strings.HasPrefix(r2.Sub, "You let them in") {
		t.Fatalf("retyped after admit: %+v", r2)
	}
	if _, err := s.RemoveGuest(aarav, g.Guests[0].ID); err == nil {
		t.Fatal("entered guest can't be removed")
	}
	stats, _ := s.Stats()
	if stats.Entered != 2 || stats.Members != 3 || stats.Guests != 1 {
		t.Fatalf("stats: %+v", stats)
	}
}

func TestPartialSettings(t *testing.T) {
	s, _ := setup(t)
	two := 2
	in := SettingsInput{PGP1Prefixes: []string{"P27", " f27 ", "bad prefix!"}}
	in.Limits.Faculty = &two
	out, err := s.SaveSettings(in)
	if err != nil || out.Limits != (Limits{PGP1: 0, Student: 3, Faculty: 2, Alumni: 2}) || strings.Join(out.PGP1Prefixes, ",") != "p27,f27" || out.Event.Gates != 2 {
		t.Fatalf("settings: %+v %v", out, err)
	}
	if u := signIn(t, s, "p27new@iima.ac.in", "New"); group(u) != "pgp1" {
		t.Fatalf("prefix change: %s", group(u))
	}
}

func TestConcurrentScansAdmitOnce(t *testing.T) {
	s, _ := setup(t)
	aarav := signIn(t, s, "p25aarav@iima.ac.in", "Aarav Shah")
	p := me(t, s, aarav).Pass
	payload := QRPayload(p.Key, p.ID, time.Now())
	// 20 different volunteers at 3 gates scan the same QR at the same moment.
	vols := make([]*core.Record, 20)
	for i := range vols {
		vols[i] = signIn(t, s, fmt.Sprintf("p24vol%02d@iima.ac.in", i), "Vol")
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i, v := range vols {
		wg.Add(1)
		go func(v *core.Record, gate int) {
			defer wg.Done()
			if r, err := s.Scan(v, payload, gate); err == nil && r.Outcome == "allowed" {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}(v, i%3+1)
	}
	wg.Wait()
	if allowed != 1 || s.EnteredCount() != 1 {
		t.Fatalf("allowed %d times, entered %d", allowed, s.EnteredCount())
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

func TestUndoEntry(t *testing.T) {
	s, _ := setup(t)
	admin := signIn(t, s, "admin.x@iima.ac.in", "Admin")
	admin.Set("role", "admin")
	s.app.Save(admin)
	u := signIn(t, s, "p25aarav@iima.ac.in", "Aarav")
	p := me(t, s, u).Pass
	r, err := s.Scan(admin, QRPayload(p.Key, p.ID, s.Now()), 1)
	if err != nil || r.Outcome != "allowed" {
		t.Fatalf("scan: %v %+v", err, r)
	}
	if err := s.AdminUndoEntry(p.ID); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if m := me(t, s, u); m.Pass.EnteredAt != nil {
		t.Fatalf("still entered: %+v", m.Pass)
	}
	list, _ := s.AdminPasses("", "entered", 0)
	if len(list.Items) != 0 {
		t.Fatalf("still in entered list: %d", len(list.Items))
	}
	r, _ = s.Scan(admin, QRPayload(p.Key, p.ID, s.Now()), 1)
	if r.Outcome != "allowed" {
		t.Fatalf("rescan after undo: %+v", r)
	}
}

func TestStaffGetOwnPass(t *testing.T) {
	s, _ := setup(t)
	// A non-IIMA volunteer, set up before they ever sign in.
	p, err := s.GrantAccess("helper@gmail.com", "volunteer")
	if err != nil || p.Role != "volunteer" {
		t.Fatalf("grant: %v %+v", err, p)
	}
	u := signIn(t, s, "helper@gmail.com", "Helper")
	m := me(t, s, u)
	if m.Pass == nil || m.Pass.Kind != "own" || m.Pass.TypeLabel != "Cultcomm team" || m.Pass.Key == "" {
		t.Fatalf("volunteer pass: %+v", m.Pass)
	}
	// It scans in like any other pass.
	r, _ := s.Scan(u, QRPayload(m.Pass.Key, m.Pass.ID, s.Now()), 1)
	if r.Outcome != "allowed" {
		t.Fatalf("scan: %+v", r)
	}
	// Promoting someone again doesn't create a second pass.
	admin := "admin"
	s.UpdatePerson(u.Id, PersonUpdate{Role: &admin})
	if n, _ := s.app.CountRecords("passes", nil); n != 1 {
		t.Fatalf("passes: %d", n)
	}
	// Non-IIMA people can't be given a plain pass this way.
	if _, err := s.GrantAccess("stranger@gmail.com", "member"); err == nil {
		t.Fatal("member grant for a stranger should fail")
	}
}

func TestAdminListsArePaged(t *testing.T) {
	s, _ := setup(t)
	total := AdminPageSize + 7
	for i := 0; i < total; i++ {
		signIn(t, s, fmt.Sprintf("p25user%03d@iima.ac.in", i), fmt.Sprintf("User %03d", i))
	}
	host := signIn(t, s, "p25user000@iima.ac.in", "User 000")
	if _, err := s.AddGuest(host, "Riya Patel", ""); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for offset, pages := 0, 0; ; pages++ {
		p, err := s.AdminPasses("", "own", offset)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range p.Items {
			if seen[v.ID] {
				t.Fatalf("pass %s listed twice", v.Code)
			}
			seen[v.ID] = true
		}
		offset += len(p.Items)
		if !p.More {
			if pages != 1 || len(seen) != total {
				t.Fatalf("pages=%d passes=%d", pages+1, len(seen))
			}
			break
		}
		if len(p.Items) != AdminPageSize {
			t.Fatalf("page size %d", len(p.Items))
		}
	}

	people, err := s.AdminPeople("", 0)
	if err != nil || len(people.Items) != AdminPageSize || !people.More {
		t.Fatalf("people: %v %d %v", err, len(people.Items), people.More)
	}
	if people.Items[0].Email != "p25user000@iima.ac.in" || people.Items[0].Guests != 1 {
		t.Fatalf("first person: %+v", people.Items[0])
	}
	rest, _ := s.AdminPeople("", AdminPageSize)
	if len(rest.Items) != 7 || rest.More {
		t.Fatalf("second page: %d %v", len(rest.Items), rest.More)
	}
	// A huge or odd search can't break the query: it is a bound value, capped at 100 characters.
	for _, q := range []string{strings.Repeat("x", 5000), strings.Repeat("ગ", 300), `%' || 1=1 || "_`, "a && b || (c)"} {
		if p, err := s.AdminPasses(q, "waiting", 0); err != nil || len(p.Items) != 0 {
			t.Fatalf("passes search %.20q: %v %d", q, err, len(p.Items))
		}
		if p, err := s.AdminPeople(q, 0); err != nil || len(p.Items) != 0 {
			t.Fatalf("people search %.20q: %v %d", q, err, len(p.Items))
		}
	}
	// Search still narrows across everything, not just the first page.
	found, _ := s.AdminPasses("User 04", "", 0)
	if len(found.Items) != 7 || found.More {
		t.Fatalf("search: %d", len(found.Items))
	}
}

func TestPrintedPasses(t *testing.T) {
	s, _ := setup(t)
	gate := signIn(t, s, "p24meera@iima.ac.in", "Meera")
	b, err := s.GeneratePrintPasses(40, " Support   staff ")
	if err != nil || len(b.Passes) != 40 || b.Label != "Support staff" {
		t.Fatalf("generate: %v %+v", err, b)
	}
	if b.Passes[0].Name != "Support staff 001" || b.Passes[39].Name != "Support staff 040" || !validCode(b.Passes[0].Code) {
		t.Fatalf("names: %q %q %q", b.Passes[0].Name, b.Passes[39].Name, b.Passes[0].Code)
	}
	seen := map[string]bool{}
	for _, p := range b.Passes {
		if seen[p.QR] || !strings.HasPrefix(p.QR, "G3.") {
			t.Fatalf("QR %q", p.QR)
		}
		seen[p.QR] = true
	}
	// A second run carries on numbering, and the first can be printed again unchanged.
	b2, _ := s.GeneratePrintPasses(3, "Support staff")
	if b2.Passes[0].Name != "Support staff 041" || b2.Batch == b.Batch && len(b2.Passes) != 43 {
		t.Fatalf("second run: %+v", b2.Passes[0])
	}
	again, err := s.PrintBatch(b.Batch)
	if err != nil || again.Passes[7].QR != b.Passes[7].QR {
		t.Fatalf("reprint: %v", err)
	}

	// The paper QR lets one person in, once. A photocopy is refused.
	r, _ := s.Scan(gate, b.Passes[0].QR, 1)
	if r.Outcome != "allowed" || r.TypeLabel != "Support staff" || !strings.Contains(r.Sub, "keep the paper pass") {
		t.Fatalf("scan: %+v", r)
	}
	gate2 := signIn(t, s, "p24other@iima.ac.in", "Other")
	for _, v := range []*core.Record{gate2, gate} { // another gate, or the same volunteer a second later
		if r, _ := s.Scan(v, b.Passes[0].QR, 2); r.Outcome != "used" {
			t.Fatalf("photocopy: %+v", r)
		}
	}
	// Typing the printed code works too, after the usual check.
	if r, _ := s.Scan(gate, b.Passes[1].Code, 1); r.Outcome != "check" {
		t.Fatalf("typed: %+v", r)
	}
	// Forged or altered codes are refused.
	parts := strings.Split(b.Passes[2].QR, ".")
	for _, bad := range []string{"G3." + parts[1] + ".AAAAAAAAAAAA", "G3." + parts[1], "G3.nosuchpass000." + parts[2], "G3..", b.Passes[2].QR + "x"} {
		if r, _ := s.Scan(gate, bad, 1); r.Outcome != "invalid" {
			t.Fatalf("%q: %+v", bad, r)
		}
	}
	// A phone pass can't be turned into a fixed code: screenshots still don't work.
	m := me(t, s, signIn(t, s, "p25aarav@iima.ac.in", "Aarav"))
	if r, _ := s.Scan(gate, printPayload(m.Pass.Key, m.Pass.ID), 1); r.Outcome != "invalid" {
		t.Fatalf("fixed code for a phone pass: %+v", r)
	}
	// A cancelled paper pass (lost, say) is refused.
	lost, _ := s.app.FindFirstRecordByData("passes", "code", b.Passes[5].Code)
	if err := s.AdminRevoke(lost.Id); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Scan(gate, b.Passes[5].QR, 1); r.Outcome != "revoked" {
		t.Fatalf("cancelled: %+v", r)
	}

	runs, err := s.PrintBatches()
	if err != nil || len(runs) != 2 || runs[0].Count != 3 || runs[1].Count != 40 || runs[1].Entered != 1 || runs[1].Cancelled != 1 {
		t.Fatalf("runs: %v %+v", err, runs)
	}
	if list, _ := s.AdminPasses("", "print", 0); len(list.Items) != AdminPageSize || !list.More || list.Items[0].Tone != "faculty" {
		t.Fatalf("list: %d", len(list.Items))
	}
	for _, n := range []int{0, -1, MaxPrintBatch + 1} {
		if _, err := s.GeneratePrintPasses(n, ""); err == nil {
			t.Fatalf("count %d accepted", n)
		}
	}
}

func TestAlumniBringGuestsFromTheirPassLink(t *testing.T) {
	s, _ := setup(t)
	gate := signIn(t, s, "p24meera@iima.ac.in", "Meera")
	sheet := "Name,Email,College,Guests\n" +
		"Asha Mehta,,IIM Ahmedabad Alumni,\n" + // alumni default
		"Ravi Rao,ravi.rao@gmail.com,IIM Ahmedabad Alumni,\n" + // alum who can also sign in
		"Tara Singh,,XLRI Jamshedpur,\n" + // ordinary exchange guest: no guests
		"Keerti Guest,,Prof. Keerti Sharda,1\n" // one guest, set in the sheet
	if res, err := s.ImportExchange("x.csv", []byte(sheet)); err != nil || res.Created != 4 || len(res.Errors) != 0 {
		t.Fatalf("import: %v %+v", err, res)
	}
	token := func(name string) string {
		p, err := s.app.FindFirstRecordByData("passes", "holderName", name)
		if err != nil {
			t.Fatal(err)
		}
		return p.GetString("linkToken")
	}
	asha := token("Asha Mehta")

	l, err := s.PassByLink(asha)
	if err != nil || l.Limit != 2 || l.Remaining != 2 || len(l.Guests) != 0 {
		t.Fatalf("alum link: %v %+v", err, l)
	}
	if l, _ := s.PassByLink(token("Tara Singh")); l.Limit != 0 {
		t.Fatalf("exchange guest got a limit: %d", l.Limit)
	}
	if _, err := s.AddGuestByLink(token("Tara Singh"), "Friend", ""); err == nil {
		t.Fatal("ordinary exchange guest added a guest")
	}
	if l, _ := s.PassByLink(token("Keerti Guest")); l.Limit != 1 {
		t.Fatalf("sheet limit: %d", l.Limit)
	}

	l, err = s.AddGuestByLink(asha, " Mohan  Mehta ", "")
	if err != nil || len(l.Guests) != 1 || l.Remaining != 1 {
		t.Fatalf("add: %v %+v", err, l)
	}
	g := l.Guests[0]
	if g.Kind != "guest" || g.HolderName != "Mohan Mehta" || g.Key == "" || g.IssuerName == nil || *g.IssuerName != "Asha Mehta" {
		t.Fatalf("guest: %+v", g)
	}
	l, _ = s.AddGuestByLink(asha, "Leela Mehta", "")
	if _, err := s.AddGuestByLink(asha, "One Too Many", ""); err == nil || l.Remaining != 0 {
		t.Fatalf("limit not enforced: %v", err)
	}
	// Guests can't add guests of their own, and a guest's link can't touch its host's guests.
	guestToken := func() string { p, _ := s.app.FindRecordById("passes", g.ID); return p.GetString("linkToken") }()
	if _, err := s.AddGuestByLink(guestToken, "Chain", ""); err == nil {
		t.Fatal("a guest added a guest")
	}
	if _, err := s.RemoveGuestByLink(token("Ravi Rao"), g.ID); err == nil {
		t.Fatal("removed someone else's guest")
	}

	// At the gate the guest scans like any other, and shows whose guest they are.
	r, _ := s.Scan(gate, QRPayload(g.Key, g.ID, s.Now()), 1)
	if r.Outcome != "allowed" || !strings.Contains(r.Meta, "Guest of Asha Mehta") {
		t.Fatalf("scan: %+v", r)
	}
	if _, err := s.RemoveGuestByLink(asha, g.ID); err == nil {
		t.Fatal("removed a guest who already entered")
	}
	l, err = s.RemoveGuestByLink(asha, l.Guests[1].ID)
	if err != nil || len(l.Guests) != 1 || l.Remaining != 1 {
		t.Fatalf("remove: %v %+v", err, l)
	}

	// An alum who signs in with Google gets the same thing from their account.
	ravi := signIn(t, s, "ravi.rao@gmail.com", "Ravi Rao")
	m := me(t, s, ravi)
	if m.Limit != 2 || m.Pass == nil || m.Pass.Kind != "exchange" {
		t.Fatalf("signed-in alum: %+v", m)
	}
	mp, err := s.AddGuest(ravi, "Sita Rao", "")
	if err != nil || len(mp.Guests) != 1 || mp.Remaining != 1 {
		t.Fatalf("signed-in add: %v", err)
	}
	// ...and sees the same guests through the link.
	if l, _ := s.PassByLink(token("Ravi Rao")); len(l.Guests) != 1 || l.Guests[0].HolderName != "Sita Rao" {
		t.Fatalf("link after signed-in add: %+v", l.Guests)
	}
	if link, err := s.GuestLink(ravi, mp.Guests[0].ID, "https://x"); err != nil || !strings.HasPrefix(link, "https://x/p/") {
		t.Fatalf("guest link: %v", err)
	}
	if mp, err = s.RemoveGuest(ravi, mp.Guests[0].ID); err != nil || len(mp.Guests) != 0 {
		t.Fatalf("signed-in remove: %v", err)
	}
	if _, err := s.RemoveGuest(ravi, g.ID); err == nil {
		t.Fatal("removed another alum's guest")
	}

	// The default is a setting; members' own limits are untouched.
	zero := 0
	in := SettingsInput{}
	in.Limits.Alumni = &zero
	if _, err := s.SaveSettings(in); err != nil {
		t.Fatal(err)
	}
	if l, _ := s.PassByLink(asha); l.Limit != 0 {
		t.Fatalf("alumni limit 0: %d", l.Limit)
	}
	if m := me(t, s, signIn(t, s, "p25aarav@iima.ac.in", "Aarav")); m.Limit != 3 {
		t.Fatalf("member limit: %d", m.Limit)
	}
}

func TestPrintedPassDefaultLabel(t *testing.T) {
	s, _ := setup(t)
	b, err := s.GeneratePrintPasses(2, "  ")
	if err != nil || b.Label != "Offline pass" || b.Passes[1].Name != "Offline pass 002" {
		t.Fatalf("default label: %v %+v", err, b)
	}
}
