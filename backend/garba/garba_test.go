package garba

import (
	"strings"
	"sync"
	"testing"

	"github.com/pocketbase/pocketbase/core"

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
	s := New(app)
	s.Mail = func(to, subject, text string) {}
	res, err := s.ImportPeople(`email,name,group,college,role
p25aarav@iima.ac.in,Aarav Shah,PGP1,,
p24meera@iima.ac.in,Meera Iyer,pgp2,,volunteer
ishaan.m@spjimr.org,Ishaan Mehta,exchange,SPJIMR Mumbai,
bad-email,Nope,pgp1,,
x@iima.ac.in,No Group,wizard,,
y@nmims.edu,No College,exchange,,
`)
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 3 || len(res.Errors) != 3 {
		t.Fatalf("import: %+v", res)
	}
	return s, app
}

func user(t *testing.T, app core.App, email string) *core.Record {
	t.Helper()
	u, err := app.FindAuthRecordByEmail("users", email)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestImportGivesOwnPasses(t *testing.T) {
	s, app := setup(t)
	me, err := s.Me(user(t, app, "p25aarav@iima.ac.in"))
	if err != nil {
		t.Fatal(err)
	}
	if me.Pass == nil || me.Pass.Kind != "own" || me.Pass.TypeLabel != "PGP1" || !strings.HasPrefix(me.Pass.QR, qrPrefix) || me.Quota != 4 || me.Remaining != 4 {
		t.Fatalf("unexpected me: %+v %+v", me, me.Pass)
	}
	guest, _ := s.Me(user(t, app, "ishaan.m@spjimr.org"))
	if guest.Pass == nil || guest.Pass.Kind != "exchange" || !strings.HasPrefix(guest.Pass.Code, "GRB-X-") || guest.Quota != 0 {
		t.Fatalf("unexpected exchange pass: %+v", guest.Pass)
	}
	// Re-import updates instead of duplicating.
	res, err := s.ImportPeople("email,name,group\np25aarav@iima.ac.in,Aarav S. Shah,pgp2\n")
	if err != nil || res.Updated != 1 || res.Created != 0 {
		t.Fatalf("reimport: %+v %v", res, err)
	}
	me, _ = s.Me(user(t, app, "p25aarav@iima.ac.in"))
	if me.Pass.HolderName != "Aarav S. Shah" || me.Pass.TypeLabel != "PGP2" {
		t.Fatalf("reimport not applied: %+v", me.Pass)
	}
}

func TestSendClaimScan(t *testing.T) {
	s, app := setup(t)
	aarav := user(t, app, "p25aarav@iima.ac.in")
	meera := user(t, app, "p24meera@iima.ac.in")

	byPhone, err := s.SendPass(aarav, "Kabir  Desai", "+91 98250 11223", "https://garba.test")
	if err != nil {
		t.Fatal(err)
	}
	if byPhone.Me.Remaining != 3 || byPhone.Pass.Status != "SENT" || !strings.HasPrefix(byPhone.ClaimURL, "https://garba.test/claim/") {
		t.Fatalf("send: %+v", byPhone)
	}
	if _, err := s.SendPass(aarav, "Kabir Desai", "+919825011223", "x"); err == nil {
		t.Fatal("expected duplicate contact to fail")
	}
	if _, err := s.SendPass(aarav, "Meera", "p24meera@iima.ac.in", "x"); err == nil {
		t.Fatal("expected sending to a roster member to fail")
	}

	// Unclaimed passes can't enter.
	token := byPhone.ClaimURL[strings.LastIndex(byPhone.ClaimURL, "/")+1:]
	r, _ := s.Scan(meera, byPhone.Pass.Code, 1)
	if r.Outcome != "unclaimed" {
		t.Fatalf("scan unclaimed: %+v", r)
	}
	info, err := s.Claim(token, "Kabir Desai")
	if err != nil || !info.Claimed || info.Pass.QR == "" {
		t.Fatalf("claim: %+v %v", info, err)
	}
	if err := s.RevokeSent(aarav, byPhone.Pass.ID); err == nil {
		t.Fatal("claimed pass should not be revocable")
	}

	r, _ = s.Scan(meera, info.Pass.QR, 2)
	if r.Outcome != "allowed" || r.Name != "Kabir Desai" || !strings.Contains(r.Meta, "Guest of Aarav Shah") || r.Entered != 1 {
		t.Fatalf("scan allowed: %+v", r)
	}
	r, _ = s.Scan(meera, info.Pass.QR, 1)
	if r.Outcome != "used" || !strings.Contains(r.Sub, "Gate 2") {
		t.Fatalf("scan used: %+v", r)
	}
	r, _ = s.Scan(meera, "https://example.com", 1)
	if r.Outcome != "invalid" {
		t.Fatalf("scan invalid: %+v", r)
	}

	// Email recipients get an account and see the pass when they sign in.
	byMail, err := s.SendPass(aarav, "Riya Patel", "Riya.Patel@gmail.com", "x")
	if err != nil {
		t.Fatal(err)
	}
	riya := user(t, app, "riya.patel@gmail.com")
	rm, _ := s.Me(riya)
	if rm.Pass == nil || rm.Pass.ID != byMail.Pass.ID || rm.Pass.Status != "CLAIMED" || rm.Quota != 0 {
		t.Fatalf("guest me: %+v", rm.Pass)
	}

	// Take back an unclaimed pass frees the slot.
	p3, _ := s.SendPass(aarav, "Nisha Joshi", "9876543210", "x")
	if err := s.RevokeSent(aarav, p3.Pass.ID); err != nil {
		t.Fatal(err)
	}
	me, _ := s.Me(aarav)
	if me.Remaining != 2 || len(me.Sent) != 2 {
		t.Fatalf("after revoke: remaining=%d sent=%d", me.Remaining, len(me.Sent))
	}

	stats, err := s.Stats()
	if err != nil || stats.Entered != 1 || stats.Members != 2 || stats.Exchange != 1 || stats.PendingClaims != 0 {
		t.Fatalf("stats: %+v %v", stats, err)
	}
	if list, _ := s.AdminPasses("aarav", ""); len(list) != 4 { // own pass + 3 sent incl. the revoked one (issuer name match)
		t.Fatalf("admin search: %d", len(list))
	}
	if csv, err := s.ExportCSV(); err != nil || !strings.Contains(string(csv), "Kabir Desai") {
		t.Fatalf("export: %v", err)
	}
}

func TestQuotaAndConcurrentScan(t *testing.T) {
	s, app := setup(t)
	aarav := user(t, app, "p25aarav@iima.ac.in")
	if _, err := s.SaveSettings(Settings{Quotas: map[string]int{"pgp1": 1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SendPass(aarav, "One Friend", "9000000001", "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SendPass(aarav, "Two Friend", "9000000002", "x"); err == nil {
		t.Fatal("expected quota error")
	}

	// Many volunteers scanning the same pass at once: exactly one entry.
	meera := user(t, app, "p24meera@iima.ac.in")
	me, _ := s.Me(aarav)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := s.Scan(meera, me.Pass.QR, 1)
			if err == nil && r.Outcome == "allowed" {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 1 {
		t.Fatalf("allowed %d times", allowed)
	}
}
