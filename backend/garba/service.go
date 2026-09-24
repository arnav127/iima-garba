package garba

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/security"
)

// Rotating QR: the pass shows G2.<passId>.<step>.<sig> where step = unix seconds / QRStep and
// sig = HMAC-SHA256(secret, "<passId>.<step>") truncated. Phones compute it offline; the gate
// accepts codes up to QRWindow steps old or new, so a screenshot stops working within ~30 s.
const (
	QRStep   = 15
	QRWindow = 2
	// RescanGraceSeconds: a volunteer re-scanning a pass they just admitted sees green, not "Already used".
	RescanGraceSeconds = 30
	qrPrefix           = "G2."
)

type Service struct {
	app   core.App
	cache settingsCache
	// AppURL is the public URL of the web app, used in pass links. Falls back to the request Origin.
	AppURL string
	// MemberDomains: anyone signing in with one of these email domains gets a pass.
	MemberDomains []string
	// Now is replaceable in tests.
	Now func() time.Time
}

func New(app core.App) *Service {
	return &Service{app: app, MemberDomains: []string{"iima.ac.in"}, Now: time.Now}
}

// ---------- API shapes (mirror shared/types.ts) ----------

type PassView struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Kind        string  `json:"kind"`
	Status      string  `json:"status"`
	HolderName  string  `json:"holderName"`
	HolderEmail *string `json:"holderEmail"`
	TypeLabel   string  `json:"typeLabel"`
	Tone        string  `json:"tone"`
	College     *string `json:"college"`
	IssuerName  *string `json:"issuerName"`
	EnteredAt   *int64  `json:"enteredAt"`
	EnteredGate *int    `json:"enteredGate"`
	// Key for the rotating QR. Only sent to people allowed to show this pass.
	Key string `json:"key,omitempty"`
}

type Me struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Group       string `json:"group"`
	Role        string `json:"role"`
	GroupLocked bool   `json:"groupLocked"`
	HasLimit    bool   `json:"hasLimit"`
	GuestLimit  int    `json:"guestLimit"`
}

type MeResponse struct {
	User       Me         `json:"user"`
	Event      EventInfo  `json:"event"`
	Pass       *PassView  `json:"pass"`
	Guests     []PassView `json:"guests"`
	Limit      int        `json:"limit"`
	Remaining  int        `json:"remaining"`
	ServerTime int64      `json:"serverTime"`
}

type LinkResponse struct {
	Event      EventInfo `json:"event"`
	Pass       PassView  `json:"pass"`
	ServerTime int64     `json:"serverTime"`
}

type ScanResult struct {
	Outcome   string `json:"outcome"` // allowed | used | revoked | invalid | expired | check
	Title     string `json:"title"`
	Sub       string `json:"sub"`
	Name      string `json:"name"`
	Meta      string `json:"meta"`
	Tone      string `json:"tone"`
	TypeLabel string `json:"typeLabel"`
	PassID    string `json:"passId,omitempty"`
	Entered   int    `json:"entered"`
}

// ---------- helpers ----------

func apiErr(status int, msg string) *router.ApiError { return router.NewApiError(status, msg, nil) }

var (
	spaceRe   = regexp.MustCompile(`\s+`)
	studentRe = regexp.MustCompile(`^[a-z]{1,6}\d{2}`)
	ist       = time.FixedZone("IST", 5*3600+1800)
)

func isEmail(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s && strings.Contains(s[strings.LastIndex(s, "@"):], ".")
}

func cleanName(s string) string { return strings.TrimSpace(spaceRe.ReplaceAllString(s, " ")) }

func (s *Service) nowMs() int64 { return s.Now().UnixMilli() }

func istTime(ms int64) string { return strings.ToUpper(time.UnixMilli(ms).In(ist).Format("3:04 PM")) }

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func role(u *core.Record) string  { return u.GetString("role") }
func group(u *core.Record) string { return u.GetString("group") }

func isMemberGroup(g string) bool { return g == "pgp1" || g == "student" || g == "faculty" }

func (s *Service) isMemberEmail(email string) bool {
	for _, d := range s.MemberDomains {
		if strings.HasSuffix(email, "@"+d) {
			return true
		}
	}
	return false
}

// cohortFor classifies an IIMA address: PGP1 prefixes (p26, f26) → pgp1; roll-number style
// addresses (letters + two digits, e.g. p25aarav, phd23x) → student; anything else → faculty & staff.
func (s *Service) cohortFor(email string) string {
	local := strings.SplitN(email, "@", 2)[0]
	for _, p := range s.Settings().PGP1Prefixes {
		if p != "" && strings.HasPrefix(local, p) {
			return "pgp1"
		}
	}
	if studentRe.MatchString(local) {
		return "student"
	}
	return "faculty"
}

func toMe(u *core.Record) Me {
	return Me{ID: u.Id, Email: u.Email(), Name: u.GetString("name"), Group: group(u), Role: role(u),
		GroupLocked: u.GetBool("groupLocked"), HasLimit: u.GetBool("hasLimit"), GuestLimit: u.GetInt("guestLimit")}
}

// GuestLimit is how many guests a person may add: their own override, else their group's default.
func (s *Service) GuestLimit(u *core.Record) int {
	if u.GetBool("hasLimit") {
		return u.GetInt("guestLimit")
	}
	l := s.Settings().Limits
	switch group(u) {
	case "pgp1":
		return l.PGP1
	case "student":
		return l.Student
	case "faculty":
		return l.Faculty
	}
	return 0
}

func (s *Service) view(p *core.Record, withKey bool) PassView {
	holder, issuer := p.ExpandedOne("holder"), p.ExpandedOne("issuer")
	v := PassView{
		ID: p.Id, Code: p.GetString("code"), Kind: p.GetString("kind"), Status: p.GetString("status"),
		HolderName: p.GetString("holderName"), HolderEmail: strPtr(p.GetString("holderEmail")),
		College: strPtr(p.GetString("college")),
	}
	switch v.Kind {
	case "guest":
		v.TypeLabel, v.Tone = "Guest", "guest"
	case "exchange":
		v.TypeLabel, v.Tone = "Exchange", "exchange"
	default:
		g := "faculty"
		if holder != nil {
			g = group(holder)
		}
		v.TypeLabel, v.Tone = GroupLabel[g], "student"
		if g == "faculty" {
			v.Tone = "faculty"
		}
	}
	if issuer != nil {
		v.IssuerName = strPtr(issuer.GetString("name"))
	}
	if at := int64(p.GetFloat("enteredAt")); at > 0 {
		gate := p.GetInt("enteredGate")
		v.EnteredAt, v.EnteredGate = &at, &gate
	}
	if withKey {
		v.Key = p.GetString("secret")
	}
	return v
}

func (s *Service) views(app core.App, recs []*core.Record, withKey bool) []PassView {
	app.ExpandRecords(recs, []string{"holder", "issuer"}, nil)
	out := make([]PassView, 0, len(recs))
	for _, p := range recs {
		out = append(out, s.view(p, withKey))
	}
	return out
}

func (s *Service) viewOne(app core.App, p *core.Record, withKey bool) PassView {
	return s.views(app, []*core.Record{p}, withKey)[0]
}

// newPass saves a pass with a fresh random code (seq only keeps creation order). Run inside a transaction.
func newPass(app core.App, kind, holderID, name, email, issuerID string, extra map[string]any) (*core.Record, error) {
	col, err := app.FindCachedCollectionByNameOrId("passes")
	if err != nil {
		return nil, err
	}
	var next struct {
		N int `db:"n"`
	}
	if err := app.DB().NewQuery("SELECT COALESCE(MAX(seq), 0) + 1 AS n FROM passes").One(&next); err != nil {
		return nil, err
	}
	code := newPassCode()
	for i := 0; i < 5; i++ { // collisions are ~1 in 150,000; retry just in case
		if _, err := app.FindFirstRecordByData("passes", "code", code); err != nil {
			break
		}
		code = newPassCode()
	}
	p := core.NewRecord(col)
	p.Set("seq", next.N)
	p.Set("code", code)
	p.Set("secret", security.RandomString(32))
	p.Set("linkToken", security.RandomString(24))
	p.Set("kind", kind)
	p.Set("status", "ACTIVE")
	p.Set("holder", holderID)
	p.Set("holderName", name)
	p.Set("holderEmail", email)
	p.Set("issuer", issuerID)
	for k, v := range extra {
		p.Set(k, v)
	}
	return p, app.Save(p)
}

func newUser(app core.App, email, name, grp, rl string) (*core.Record, error) {
	col, err := app.FindCachedCollectionByNameOrId("users")
	if err != nil {
		return nil, err
	}
	u := core.NewRecord(col)
	u.SetEmail(email)
	u.SetVerified(true)
	u.SetRandomPassword()
	u.Set("name", name)
	u.Set("group", grp)
	u.Set("role", rl)
	return u, app.Save(u)
}

// ensureOwnPass gives an IIMA member their own pass (once), keeping its name in sync.
func ensureOwnPass(app core.App, u *core.Record) error {
	if !isMemberGroup(group(u)) {
		return nil
	}
	p, err := app.FindFirstRecordByFilter("passes", "holder = {:u} && kind = 'own'", dbx.Params{"u": u.Id})
	if err != nil {
		_, err = newPass(app, "own", u.Id, u.GetString("name"), u.Email(), "", nil)
		return err
	}
	if p.GetString("holderName") != u.GetString("name") && u.GetString("name") != "" {
		p.Set("holderName", u.GetString("name"))
		return app.Save(p)
	}
	return nil
}

// linkPasses attaches guest/exchange passes that were added for this person's email.
func linkPasses(app core.App, u *core.Record) error {
	recs, err := app.FindRecordsByFilter("passes", "holderEmail = {:e} && holder = '' && kind != 'own'", "", 0, 0, dbx.Params{"e": u.Email()})
	if err != nil {
		return err
	}
	for _, p := range recs {
		p.Set("holder", u.Id)
		if err := app.Save(p); err != nil {
			return err
		}
	}
	return nil
}

// ---------- sign-in ----------

var ErrNotOnList = apiErr(http.StatusForbidden, "This account isn't on the Garba Night list. IIMA students, faculty and staff: sign in with your @iima.ac.in account. Guests: use the Gmail your host added, or ask them for your pass link.")

// SignInUser finds or creates the account for a verified OAuth2 email.
// IIMA addresses always get in (with their own pass); anyone else needs a pass added for their email.
func (s *Service) SignInUser(emailRaw, name string) (*core.Record, error) {
	email := strings.ToLower(strings.TrimSpace(emailRaw))
	if email == "" {
		return nil, ErrNotOnList
	}
	name = cleanName(name)
	if name == "" {
		name = strings.SplitN(email, "@", 2)[0]
	}
	var user *core.Record
	err := s.app.RunInTransaction(func(tx core.App) error {
		u, err := tx.FindAuthRecordByEmail("users", email)
		member := s.isMemberEmail(email)
		switch {
		case err == nil:
			if member && !u.GetBool("groupLocked") && group(u) != s.cohortFor(email) {
				u.Set("group", s.cohortFor(email))
				if err := tx.Save(u); err != nil {
					return err
				}
			}
		case member:
			if u, err = newUser(tx, email, name, s.cohortFor(email), "member"); err != nil {
				return err
			}
		default:
			held, err := tx.FindRecordsByFilter("passes", "holderEmail = {:e} && kind != 'own' && status = 'ACTIVE'", "", 0, 0, dbx.Params{"e": email})
			if err != nil || len(held) == 0 {
				return ErrNotOnList
			}
			grp := "guest"
			for _, p := range held {
				if p.GetString("kind") == "exchange" {
					grp = "exchange"
				}
			}
			if u, err = newUser(tx, email, held[0].GetString("holderName"), grp, "member"); err != nil {
				return err
			}
		}
		if err := ensureOwnPass(tx, u); err != nil {
			return err
		}
		if err := linkPasses(tx, u); err != nil {
			return err
		}
		user = u
		return nil
	})
	return user, err
}

// ---------- member ----------

func (s *Service) Me(u *core.Record) (MeResponse, error) {
	app := s.app
	if fresh, err := app.FindRecordById("users", u.Id); err == nil {
		u = fresh
	}
	res := MeResponse{User: toMe(u), Event: s.Settings().Event, Guests: []PassView{}, ServerTime: s.nowMs()}
	if !isMemberGroup(group(u)) {
		if err := linkPasses(app, u); err != nil {
			return res, err
		}
	}
	own, err := app.FindFirstRecordByFilter("passes", "holder = {:u} && kind = 'own'", dbx.Params{"u": u.Id})
	if err != nil {
		own, err = app.FindFirstRecordByFilter("passes", "holder = {:u} && kind != 'own' && status = 'ACTIVE'", dbx.Params{"u": u.Id})
	}
	if err == nil {
		v := s.viewOne(app, own, true)
		res.Pass = &v
	}
	guests, err := app.FindRecordsByFilter("passes", "issuer = {:u} && status = 'ACTIVE'", "seq", 0, 0, dbx.Params{"u": u.Id})
	if err != nil {
		return res, err
	}
	res.Guests = s.views(app, guests, true)
	res.Limit = s.GuestLimit(u)
	res.Remaining = max(res.Limit-len(guests), 0)
	return res, nil
}

func (s *Service) AddGuest(u *core.Record, nameRaw, emailRaw string) (*MeResponse, error) {
	name := cleanName(nameRaw)
	email := strings.ToLower(strings.TrimSpace(emailRaw))
	if len(name) < 2 || len(name) > 80 {
		return nil, apiErr(400, "Enter their name as on their ID")
	}
	if email != "" && !isEmail(email) {
		return nil, apiErr(400, "That email doesn't look right. Leave it empty to keep their pass on your phone.")
	}
	if !isMemberGroup(group(u)) {
		return nil, apiErr(403, "Only IIMA members can add guests")
	}
	err := s.app.RunInTransaction(func(tx core.App) error {
		// Re-read the person so a limit an admin just changed applies.
		if fresh, err := tx.FindRecordById("users", u.Id); err == nil {
			u = fresh
		}
		n, err := tx.CountRecords("passes", dbx.HashExp{"issuer": u.Id, "status": "ACTIVE"})
		if err != nil {
			return err
		}
		if int(n) >= s.GuestLimit(u) {
			if s.GuestLimit(u) == 0 {
				return apiErr(409, "Your pass is for yourself only")
			}
			return apiErr(409, "You've added all your guests")
		}
		holderID := ""
		if email != "" {
			if email == u.Email() {
				return apiErr(400, "That's your own email")
			}
			if s.isMemberEmail(email) {
				return apiErr(409, "IIMA members get their own pass when they sign in")
			}
			if dup, err := tx.FindFirstRecordByFilter("passes", "holderEmail = {:e} && status = 'ACTIVE'", dbx.Params{"e": email}); err == nil {
				tx.ExpandRecord(dup, []string{"issuer"}, nil)
				by := "Cultcomm"
				if is := dup.ExpandedOne("issuer"); is != nil {
					by = is.GetString("name")
				}
				return apiErr(409, fmt.Sprintf("%s already has a pass from %s", email, by))
			}
			if existing, err := tx.FindAuthRecordByEmail("users", email); err == nil {
				holderID = existing.Id
			}
		}
		_, err = newPass(tx, "guest", holderID, name, email, u.Id, nil)
		return err
	})
	if err != nil {
		return nil, err
	}
	me, err := s.Me(u)
	return &me, err
}

func (s *Service) myGuest(tx core.App, u *core.Record, id string) (*core.Record, error) {
	p, err := tx.FindFirstRecordByFilter("passes", "id = {:id} && issuer = {:u} && status = 'ACTIVE'", dbx.Params{"id": id, "u": u.Id})
	if err != nil {
		return nil, apiErr(404, "Pass not found")
	}
	return p, nil
}

func (s *Service) RemoveGuest(u *core.Record, id string) (*MeResponse, error) {
	err := s.app.RunInTransaction(func(tx core.App) error {
		p, err := s.myGuest(tx, u, id)
		if err != nil {
			return err
		}
		if p.GetFloat("enteredAt") > 0 {
			return apiErr(409, "They've already entered, so this pass can't be removed")
		}
		p.Set("status", "REVOKED")
		p.Set("revokedAt", s.nowMs())
		return tx.Save(p)
	})
	if err != nil {
		return nil, err
	}
	me, err := s.Me(u)
	return &me, err
}

func (s *Service) GuestLink(u *core.Record, id, appURL string) (string, error) {
	p, err := s.myGuest(s.app, u, id)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(appURL, "/") + "/p/" + p.GetString("linkToken"), nil
}

// PassByLink serves /p/<token>: the pass, with its QR key, for people without a Google account.
func (s *Service) PassByLink(token string) (*LinkResponse, error) {
	if len(token) < 16 {
		return nil, apiErr(404, "This pass link isn't valid")
	}
	p, err := s.app.FindFirstRecordByData("passes", "linkToken", token)
	if err != nil || p.GetString("status") != "ACTIVE" {
		return nil, apiErr(404, "This pass link isn't valid any more")
	}
	return &LinkResponse{Event: s.Settings().Event, Pass: s.viewOne(s.app, p, true), ServerTime: s.nowMs()}, nil
}

// ---------- gate ----------

func qrSig(secret, passID string, step int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(passID + "." + strconv.FormatInt(step, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:9])
}

// QRPayload is what a phone shows right now (also used by tests).
func QRPayload(secret, passID string, at time.Time) string {
	step := at.Unix() / QRStep
	return fmt.Sprintf("%s%s.%d.%s", qrPrefix, passID, step, qrSig(secret, passID, step))
}

func (s *Service) EnteredCount() int {
	n, _ := s.app.CountRecords("passes", dbx.NewExp("enteredAt > 0"))
	return int(n)
}

func (s *Service) passMeta(v PassView) string {
	switch v.Kind {
	case "guest":
		by := "a member"
		if v.IssuerName != nil {
			by = *v.IssuerName
		}
		return fmt.Sprintf("Guest of %s · %s", by, v.Code)
	case "exchange":
		college := "Pass exchange"
		if v.College != nil {
			college = *v.College
		}
		return fmt.Sprintf("%s · %s", college, v.Code)
	}
	return fmt.Sprintf("%s · Own pass · %s", v.TypeLabel, v.Code)
}

func idCheck(v PassView) string {
	switch v.Tone {
	case "guest":
		return "check photo ID"
	case "exchange":
		return "check college ID"
	case "faculty":
		return "check IIMA ID"
	}
	return "check student ID"
}

func (s *Service) result(outcome, title, sub string, v *PassView) *ScanResult {
	r := &ScanResult{Outcome: outcome, Title: title, Sub: sub, Name: "Unknown pass", Entered: s.EnteredCount()}
	if v != nil {
		r.Name, r.Meta, r.Tone, r.TypeLabel = v.HolderName, s.passMeta(*v), v.Tone, v.TypeLabel
	}
	return r
}

// Scan checks a QR from the camera (rotating G2 code) or a pass code typed by the volunteer.
func (s *Service) Scan(volunteer *core.Record, payloadRaw string, gate int) (*ScanResult, error) {
	payload := strings.TrimSpace(payloadRaw)

	if looksLikePassCode(payload) {
		code, ok := normalizePassCode(payload)
		if !ok {
			return s.result("invalid", "Code doesn't check out", "Probably a typo. Read it again, or it isn't a real pass", nil), nil
		}
		p, err := s.app.FindFirstRecordByData("passes", "code", code)
		if err != nil {
			return s.result("invalid", "Not a valid pass", "No pass has this code", nil), nil
		}
		v := s.viewOne(s.app, p, false)
		if r := s.stateCheck(p, v); r != nil {
			return s.graceRescan(volunteer, p, v, r), nil
		}
		r := s.result("check", "Check ID first", "Typed code: "+idCheck(v)+", then admit", &v)
		r.PassID = p.Id
		return r, nil
	}

	parts := strings.Split(strings.TrimPrefix(payload, qrPrefix), ".")
	if !strings.HasPrefix(payload, qrPrefix) || len(parts) != 3 {
		return s.result("invalid", "Not a valid pass", "This QR isn't a Garba Night pass. No entry.", nil), nil
	}
	p, err := s.app.FindRecordById("passes", parts[0])
	if err != nil {
		return s.result("invalid", "Not a valid pass", "This QR isn't a Garba Night pass. No entry.", nil), nil
	}
	step, _ := strconv.ParseInt(parts[1], 10, 64)
	v := s.viewOne(s.app, p, false)
	if !hmac.Equal([]byte(parts[2]), []byte(qrSig(p.GetString("secret"), p.Id, step))) {
		return s.result("invalid", "Not a valid pass", "This QR has been tampered with. No entry.", nil), nil
	}
	if d := s.Now().Unix()/QRStep - step; d > QRWindow || d < -QRWindow {
		return s.result("expired", "Old QR · screenshot?", "Ask them to open the live pass on their phone", &v), nil
	}
	return s.admit(volunteer, p, v, gate)
}

// Admit lets a pass in after a volunteer has checked ID for a typed code.
func (s *Service) Admit(volunteer *core.Record, passID string, gate int) (*ScanResult, error) {
	p, err := s.app.FindRecordById("passes", passID)
	if err != nil {
		return nil, apiErr(404, "Pass not found")
	}
	return s.admit(volunteer, p, s.viewOne(s.app, p, false), gate)
}

func (s *Service) stateCheck(p *core.Record, v PassView) *ScanResult {
	if p.GetString("status") == "REVOKED" {
		return s.result("revoked", "Pass cancelled", "This pass was removed. No entry.", &v)
	}
	if at := int64(p.GetFloat("enteredAt")); at > 0 {
		return s.result("used", "Already used", fmt.Sprintf("Scanned at %s · Gate %d", istTime(at), p.GetInt("enteredGate")), &v)
	}
	return nil
}

// graceRescan turns "Already used" green when the same volunteer let this pass in moments ago,
// usually because the reply was lost on a bad network and they scanned or typed it again.
func (s *Service) graceRescan(volunteer, p *core.Record, v PassView, r *ScanResult) *ScanResult {
	ago := (s.nowMs() - int64(p.GetFloat("enteredAt"))) / 1000
	if r.Outcome == "used" && p.GetString("enteredBy") == volunteer.Id && ago >= 0 && ago < RescanGraceSeconds {
		return s.result("allowed", "Entry allowed", fmt.Sprintf("You let them in %ds ago · %s", ago, idCheck(v)), &v)
	}
	return r
}

func (s *Service) admit(volunteer *core.Record, p *core.Record, v PassView, gateRaw int) (*ScanResult, error) {
	gate := min(max(gateRaw, 1), s.Settings().Event.Gates)
	var blocked *ScanResult
	err := s.app.RunInTransaction(func(tx core.App) error {
		fresh, err := tx.FindRecordById("passes", p.Id)
		if err != nil {
			return err
		}
		if blocked = s.stateCheck(fresh, v); blocked != nil {
			blocked = s.graceRescan(volunteer, fresh, v, blocked)
			return nil
		}
		fresh.Set("enteredAt", s.nowMs())
		fresh.Set("enteredGate", gate)
		fresh.Set("enteredBy", volunteer.Id)
		return tx.Save(fresh)
	})
	if err != nil {
		return nil, err
	}
	if blocked != nil {
		return blocked, nil
	}
	sub := "Welcome, khelaiya! · " + idCheck(v)
	return s.result("allowed", "Entry allowed", sub, &v), nil
}

// RequireRole allows only users whose role is one of roles.
func RequireRole(roles ...string) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return apiErr(http.StatusUnauthorized, "Please sign in")
		}
		for _, r := range roles {
			if role(e.Auth) == r {
				return e.Next()
			}
		}
		return apiErr(http.StatusForbidden, "Not allowed")
	}
}
