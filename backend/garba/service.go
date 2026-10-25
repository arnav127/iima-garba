package garba

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"slices"
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
	Event EventInfo `json:"event"`
	Pass  PassView  `json:"pass"`
	// Guests this pass holder has added (alumni may bring family), and how many they may add.
	Guests     []PassView `json:"guests"`
	Limit      int        `json:"limit"`
	Remaining  int        `json:"remaining"`
	ServerTime int64      `json:"serverTime"`
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
	studentRe = regexp.MustCompile(`^[a-z]{1,10}\d{2}`) // longest programme prefix so far: bpgpbaai26…
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

// isStaff: gate volunteers and Cultcomm admins, who get their own pass even without an IIMA address.
func isStaff(u *core.Record) bool { return role(u) == "volunteer" || role(u) == "admin" }

func (s *Service) isMemberEmail(email string) bool {
	for _, d := range s.MemberDomains {
		if strings.HasSuffix(email, "@"+d) {
			return true
		}
	}
	return false
}

// cohortFor classifies an IIMA address: PGP1 prefixes (p26, f26) → pgp1; roll-number style
// addresses (up to 10 letters + two digits, e.g. p25aarav, phd23x, bpgpbaai26y) → student; anything else → faculty & staff.
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
	holder, issuer, host := p.ExpandedOne("holder"), p.ExpandedOne("issuer"), p.ExpandedOne("host")
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
	case "print": // a paper pass; its label ("Offline pass") is kept in college
		v.TypeLabel, v.Tone = "Printed pass", "faculty"
		if v.College != nil {
			v.TypeLabel = *v.College
		}
	default:
		g := "faculty"
		if holder != nil {
			g = group(holder)
		}
		switch {
		case g == "faculty":
			v.TypeLabel, v.Tone = GroupLabel[g], "faculty"
		case isMemberGroup(g):
			v.TypeLabel, v.Tone = GroupLabel[g], "student"
		default: // a volunteer or admin without an IIMA address
			v.TypeLabel, v.Tone = "Cultcomm team", "guest"
		}
	}
	if issuer != nil {
		v.IssuerName = strPtr(issuer.GetString("name"))
	} else if host != nil {
		v.IssuerName = strPtr(host.GetString("holderName"))
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
	app.ExpandRecords(recs, []string{"holder", "issuer", "host"}, nil)
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

// ensureOwnPass gives an IIMA member, volunteer or admin their own pass (once), keeping its name in sync.
func ensureOwnPass(app core.App, u *core.Record) error {
	if !isMemberGroup(group(u)) && !isStaff(u) {
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

// Person is what the platform says about someone signing in.
type Person struct {
	Email, Name string
	Member      bool   // a verified @iima.ac.in Google account
	Cohort      string // pgp1, student or faculty for members (see Cohort)
	Role        string // member, volunteer or admin, from the platform's roles for garba2026; "" leaves it as is
}

// Cohort maps the platform's attributes onto Garba's groups: first-year PGP and PGP-FABM are pgp1,
// other students (and alumni, who keep their address) are student, everyone else is faculty & staff.
// Admins can still move someone in the People tab, which locks their group.
func Cohort(affiliation, programme string, year int) string {
	switch affiliation {
	case "student", "alum":
		if year == 1 && (programme == "PGP" || programme == "PGP-FABM") {
			return "pgp1"
		}
		return "student"
	}
	return "faculty"
}

// RoleFrom picks Garba's role from the platform's roles for this app.
func RoleFrom(roles []string) string {
	switch {
	case slices.Contains(roles, "admin"):
		return "admin"
	case slices.Contains(roles, "volunteer"):
		return "volunteer"
	}
	return "member"
}

// Access is called by the platform kit on every request from a signed-in user. IIMA members always
// get in (with their own pass); volunteers and admins get in with a pass of their own even without
// an IIMA address; anyone else needs a pass added for their email. Returns nil, nil for someone
// who isn't on the list, so the request runs signed out.
func (s *Service) Access(p Person) (*core.Record, error) {
	u, err := s.signIn(p)
	if errors.Is(err, ErrNotOnList) {
		return nil, nil
	}
	return u, err
}

// SignInUser signs in from an email alone (tests and tools): membership and cohort come from the
// address, and the role is left as it is.
func (s *Service) SignInUser(emailRaw, name string) (*core.Record, error) {
	email := strings.ToLower(strings.TrimSpace(emailRaw))
	return s.signIn(Person{Email: email, Name: name, Member: s.isMemberEmail(email), Cohort: s.cohortFor(email)})
}

func (s *Service) signIn(p Person) (*core.Record, error) {
	email := strings.ToLower(strings.TrimSpace(p.Email))
	if email == "" {
		return nil, ErrNotOnList
	}
	name := cleanName(p.Name)
	if name == "" {
		name = strings.SplitN(email, "@", 2)[0]
	}
	staff := p.Role == "volunteer" || p.Role == "admin"
	var user *core.Record
	err := s.app.RunInTransaction(func(tx core.App) error {
		u, err := tx.FindAuthRecordByEmail("users", email)
		switch {
		case err == nil:
			changed := false
			if p.Member && p.Cohort != "" && !u.GetBool("groupLocked") && group(u) != p.Cohort {
				u.Set("group", p.Cohort)
				changed = true
			}
			// The platform is the source of truth for roles; this field mirrors it.
			if p.Role != "" && role(u) != p.Role {
				u.Set("role", p.Role)
				changed = true
			}
			if changed {
				if err := tx.Save(u); err != nil {
					return err
				}
			}
		case p.Member:
			if u, err = newUser(tx, email, name, p.Cohort, orMember(p.Role)); err != nil {
				return err
			}
		case staff:
			// A volunteer or admin without an IIMA address.
			if u, err = newUser(tx, email, name, "guest", p.Role); err != nil {
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
	filter, params := "issuer = {:u} && status = 'ACTIVE'", dbx.Params{"u": u.Id}
	res.Limit = s.GuestLimit(u)
	if hp := s.hostPass(app, u); hp != nil { // an alum signed in with Google: guests hang off their pass
		filter, params = "host = {:h} && status = 'ACTIVE'", dbx.Params{"h": hp.Id}
		res.Limit = s.hostLimit(hp)
		if err != nil || own.GetString("kind") != "own" {
			own, err = hp, nil
		}
	}
	if err == nil {
		v := s.viewOne(app, own, true)
		res.Pass = &v
	}
	guests, err := app.FindRecordsByFilter("passes", filter, "seq", 0, 0, params)
	if err != nil {
		return res, err
	}
	res.Guests = s.views(app, guests, true)
	res.Remaining = max(res.Limit-len(guests), 0)
	return res, nil
}

func (s *Service) AddGuest(u *core.Record, nameRaw, emailRaw string) (*MeResponse, error) {
	name, email, err := cleanGuest(nameRaw, emailRaw)
	if err != nil {
		return nil, err
	}
	err = s.app.RunInTransaction(func(tx core.App) error {
		// Re-read the person so a limit an admin just changed applies.
		if fresh, err := tx.FindRecordById("users", u.Id); err == nil {
			u = fresh
		}
		if isMemberGroup(group(u)) {
			return s.addGuest(tx, name, email, guestOwner{issuer: u.Id, email: u.Email(), limit: s.GuestLimit(u)})
		}
		hp := s.hostPass(tx, u)
		if hp == nil {
			return apiErr(403, "Only IIMA members can add guests")
		}
		return s.addGuest(tx, name, email, guestOwner{host: hp.Id, email: u.Email(), limit: s.hostLimit(hp)})
	})
	if err != nil {
		return nil, err
	}
	me, err := s.Me(u)
	return &me, err
}

// myGuest finds an active guest pass that u added, as a member or from their host pass.
func (s *Service) myGuest(tx core.App, u *core.Record, id string) (*core.Record, error) {
	p, err := tx.FindFirstRecordByFilter("passes", "id = {:id} && kind = 'guest' && status = 'ACTIVE'", dbx.Params{"id": id})
	if err == nil {
		if p.GetString("issuer") == u.Id {
			return p, nil
		}
		if hp := s.hostPass(tx, u); hp != nil && p.GetString("host") == hp.Id {
			return p, nil
		}
	}
	return nil, apiErr(404, "Pass not found")
}

func (s *Service) RemoveGuest(u *core.Record, id string) (*MeResponse, error) {
	err := s.app.RunInTransaction(func(tx core.App) error {
		p, err := s.myGuest(tx, u, id)
		if err != nil {
			return err
		}
		return s.removeGuest(tx, p)
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

// PassByLink serves /p/<token>: the pass, with its QR key, for people without a Google account,
// plus any guests they have added from it.
func (s *Service) PassByLink(token string) (*LinkResponse, error) {
	p, err := s.passByToken(s.app, token)
	if err != nil {
		return nil, err
	}
	return s.linkResponse(p)
}

// ---------- gate ----------

// hmacSum signs msg with a pass's secret (HMAC-SHA256, first 9 bytes, base64url).
func hmacSum(secret, msg string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:9])
}

func hmacEqual(a, b string) bool { return hmac.Equal([]byte(a), []byte(b)) }

func qrSig(secret, passID string, step int64) string {
	return hmacSum(secret, passID+"."+strconv.FormatInt(step, 10))
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
	case "print":
		return fmt.Sprintf("Printed pass · %s", v.Code)
	}
	return fmt.Sprintf("%s · Own pass · %s", v.TypeLabel, v.Code)
}

func idCheck(v PassView) string {
	if v.Kind == "print" {
		return "keep the paper pass"
	}
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

	if strings.HasPrefix(payload, printPrefix) {
		return s.scanPrinted(volunteer, payload, gate)
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
	if !hmacEqual(parts[2], qrSig(p.GetString("secret"), p.Id, step)) {
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
	// Not for paper passes: the volunteer keeps the slip, so a second one moments later is a photocopy.
	if r.Outcome == "used" && p.GetString("kind") != "print" && p.GetString("enteredBy") == volunteer.Id && ago >= 0 && ago < RescanGraceSeconds {
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

func orMember(role string) string {
	if role == "" {
		return "member"
	}
	return role
}
