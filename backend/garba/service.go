package garba

import (
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/security"
)

const qrPrefix = "GRB1:"

type Service struct {
	app   core.App
	cache settingsCache
	// AppURL is the public URL of the web app, used in claim links. Falls back to the request Origin.
	AppURL string
	// Mail sends a plain text email. Replaced in tests.
	Mail func(to, subject, text string)
}

func New(app core.App) *Service {
	s := &Service{app: app}
	s.Mail = s.sendMail
	return s
}

// ---------- API shapes (mirror shared/types.ts) ----------

type PassView struct {
	ID            string  `json:"id"`
	Code          string  `json:"code"`
	Kind          string  `json:"kind"`
	Status        string  `json:"status"`
	HolderName    string  `json:"holderName"`
	HolderContact string  `json:"holderContact"`
	TypeLabel     string  `json:"typeLabel"`
	College       *string `json:"college"`
	IssuerName    *string `json:"issuerName"`
	EnteredAt     *int64  `json:"enteredAt"`
	EnteredGate   *int    `json:"enteredGate"`
	QR            string  `json:"qr,omitempty"`
	HolderEmail   *string `json:"holderEmail,omitempty"`
}

type Me struct {
	ID      string  `json:"id"`
	Email   string  `json:"email"`
	Name    string  `json:"name"`
	Group   string  `json:"group"`
	Role    string  `json:"role"`
	College *string `json:"college"`
}

type MeResponse struct {
	User      Me         `json:"user"`
	Event     EventInfo  `json:"event"`
	Pass      *PassView  `json:"pass"`
	Quota     int        `json:"quota"`
	Remaining int        `json:"remaining"`
	Sent      []PassView `json:"sent"`
}

type ClaimResponse struct {
	Event   EventInfo `json:"event"`
	Pass    PassView  `json:"pass"`
	Claimed bool      `json:"claimed"`
}

type ScanResult struct {
	Outcome string `json:"outcome"`
	Title   string `json:"title"`
	Sub     string `json:"sub"`
	Name    string `json:"name"`
	Meta    string `json:"meta"`
	Entered int    `json:"entered"`
}

// ---------- helpers ----------

func apiErr(status int, msg string) *router.ApiError {
	return router.NewApiError(status, msg, nil)
}

var (
	phoneRe = regexp.MustCompile(`^\+?\d{10,13}$`)
	codeRe  = regexp.MustCompile(`(?i)^GRB-(X-)?\d{1,6}$`)
	spaceRe = regexp.MustCompile(`\s+`)
	ist     = time.FixedZone("IST", 5*3600+1800)
)

func isEmail(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s && strings.Contains(s[strings.LastIndex(s, "@"):], ".")
}

func cleanName(s string) string { return strings.TrimSpace(spaceRe.ReplaceAllString(s, " ")) }

func nowMs() int64 { return time.Now().UnixMilli() }

func istTime(ms int64) string {
	return strings.ToUpper(time.UnixMilli(ms).In(ist).Format("3:04 PM"))
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func role(u *core.Record) string  { return u.GetString("role") }
func group(u *core.Record) string { return u.GetString("group") }

func toMe(u *core.Record) Me {
	return Me{ID: u.Id, Email: u.Email(), Name: u.GetString("name"), Group: group(u), Role: role(u), College: strPtr(u.GetString("college"))}
}

func (s *Service) passView(app core.App, p *core.Record, withQR bool) PassView {
	if p.Expand() == nil || (p.GetString("holder") != "" && p.ExpandedOne("holder") == nil) || (p.GetString("issuer") != "" && p.ExpandedOne("issuer") == nil) {
		app.ExpandRecord(p, []string{"holder", "issuer"}, nil)
	}
	return s.viewExpanded(p, withQR)
}

func (s *Service) viewExpanded(p *core.Record, withQR bool) PassView {
	holder, issuer := p.ExpandedOne("holder"), p.ExpandedOne("issuer")
	v := PassView{
		ID: p.Id, Code: p.GetString("code"), Kind: p.GetString("kind"), Status: p.GetString("status"),
		HolderName: p.GetString("holderName"), HolderContact: p.GetString("holderContact"),
	}
	switch v.Kind {
	case "guest":
		v.TypeLabel = "Guest"
	case "exchange":
		v.TypeLabel = "Exchange"
	default:
		v.TypeLabel = "Staff"
		if holder != nil {
			v.TypeLabel = GroupLabel[group(holder)]
		}
	}
	if holder != nil {
		v.College = strPtr(holder.GetString("college"))
		v.HolderEmail = strPtr(holder.Email())
	}
	if issuer != nil {
		v.IssuerName = strPtr(issuer.GetString("name"))
	}
	if at := int64(p.GetFloat("enteredAt")); at > 0 {
		gate := p.GetInt("enteredGate")
		v.EnteredAt, v.EnteredGate = &at, &gate
	}
	if withQR {
		v.QR = qrPrefix + p.GetString("secret")
	}
	return v
}

func (s *Service) views(app core.App, recs []*core.Record, withQR bool) []PassView {
	app.ExpandRecords(recs, []string{"holder", "issuer"}, nil)
	out := make([]PassView, 0, len(recs))
	for _, p := range recs {
		out = append(out, s.viewExpanded(p, withQR))
	}
	return out
}

func (s *Service) quotaFor(u *core.Record) int { return s.Settings().Quotas[group(u)] }

func (s *Service) activeSent(app core.App, userID string) ([]*core.Record, error) {
	return app.FindRecordsByFilter("passes", "issuer = {:u} && status != 'REVOKED'", "-seq", 0, 0, dbx.Params{"u": userID})
}

// newPass allocates the next sequence number and saves a pass. Must run inside a transaction.
func newPass(app core.App, kind, status, holderID, holderName, contact, issuerID string) (*core.Record, error) {
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
	prefix := "GRB-"
	if kind == "exchange" {
		prefix = "GRB-X-"
	}
	p := core.NewRecord(col)
	p.Set("seq", next.N)
	p.Set("code", fmt.Sprintf("%s%04d", prefix, next.N))
	p.Set("secret", security.RandomString(16))
	if kind == "guest" {
		p.Set("claimToken", security.RandomString(22))
	}
	p.Set("kind", kind)
	p.Set("status", status)
	p.Set("holder", holderID)
	p.Set("holderName", holderName)
	p.Set("holderContact", contact)
	p.Set("issuer", issuerID)
	if status == "CLAIMED" {
		p.Set("claimedAt", nowMs())
	}
	return p, app.Save(p)
}

// ensureOwnPass creates (or updates) the entry pass of a roster member. Must run inside a transaction.
func ensureOwnPass(app core.App, u *core.Record) error {
	if group(u) == "guest" {
		return nil
	}
	kind := "own"
	if group(u) == "exchange" {
		kind = "exchange"
	}
	p, err := app.FindFirstRecordByFilter("passes", "holder = {:u} && kind != 'guest'", dbx.Params{"u": u.Id})
	if err != nil {
		_, err = newPass(app, kind, "CLAIMED", u.Id, u.GetString("name"), u.Email(), "")
		return err
	}
	if p.GetString("kind") == kind && p.GetString("holderName") == u.GetString("name") {
		return nil
	}
	p.Set("kind", kind)
	p.Set("holderName", u.GetString("name"))
	return app.Save(p)
}

func newUser(app core.App, email, name, grp, rl, college string) (*core.Record, error) {
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
	u.Set("college", college)
	return u, app.Save(u)
}

func (s *Service) EventInfo() EventInfo { return s.Settings().Event }

// ---------- member ----------

func (s *Service) Me(u *core.Record) (MeResponse, error) {
	app := s.app
	res := MeResponse{User: toMe(u), Event: s.Settings().Event, Sent: []PassView{}}

	own, err := app.FindFirstRecordByFilter("passes", "holder = {:u} && kind != 'guest'", dbx.Params{"u": u.Id})
	if err != nil {
		// Friends who were sent a pass by email: show their guest pass. Signing in claims it.
		own, err = app.FindFirstRecordByFilter("passes", "holder = {:u} && kind = 'guest' && status != 'REVOKED'", dbx.Params{"u": u.Id})
		if err == nil && own.GetString("status") == "SENT" {
			own.Set("status", "CLAIMED")
			own.Set("claimedAt", nowMs())
			if err := app.Save(own); err != nil {
				return res, err
			}
		}
	}
	if own != nil && err == nil {
		v := s.passView(app, own, true)
		res.Pass = &v
	}

	sent, err := s.activeSent(app, u.Id)
	if err != nil {
		return res, err
	}
	res.Sent = s.views(app, sent, false)
	res.Quota = s.quotaFor(u)
	res.Remaining = max(res.Quota-len(sent), 0)
	return res, nil
}

type SendResult struct {
	Pass     PassView   `json:"pass"`
	ClaimURL string     `json:"claimUrl"`
	Me       MeResponse `json:"me"`
}

func (s *Service) SendPass(u *core.Record, nameRaw, contactRaw, appURL string) (*SendResult, error) {
	name := cleanName(nameRaw)
	contact := strings.TrimSpace(contactRaw)
	if len(name) < 2 || len(name) > 80 {
		return nil, apiErr(400, "Enter your friend's name as on their ID")
	}
	email := strings.ToLower(contact)
	isMail := isEmail(email)
	if !isMail {
		phone := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(contact)
		if !phoneRe.MatchString(phone) {
			return nil, apiErr(400, "Enter an email address or a 10-digit phone number")
		}
		contact = phone
	} else {
		contact = email
	}
	if s.quotaFor(u) == 0 {
		return nil, apiErr(403, "Your pass can't be shared")
	}

	var pass *core.Record
	err := s.app.RunInTransaction(func(tx core.App) error {
		sent, err := s.activeSent(tx, u.Id)
		if err != nil {
			return err
		}
		if len(sent) >= s.quotaFor(u) {
			return apiErr(409, "You've shared all your passes")
		}
		holderID := ""
		if isMail {
			if email == u.Email() {
				return apiErr(400, "You already have your own pass")
			}
			friend, err := tx.FindAuthRecordByEmail("users", email)
			if err == nil {
				if group(friend) != "guest" {
					return apiErr(409, friend.GetString("name")+" already has their own pass")
				}
			} else if friend, err = newUser(tx, email, name, "guest", "member", ""); err != nil {
				return err
			}
			holderID = friend.Id
		}
		dupFilter := "holderContact = {:c} && kind = 'guest' && status != 'REVOKED'"
		if dup, err := tx.FindFirstRecordByFilter("passes", dupFilter, dbx.Params{"c": contact}); err == nil {
			tx.ExpandRecord(dup, []string{"issuer"}, nil)
			by := "someone"
			if is := dup.ExpandedOne("issuer"); is != nil {
				by = is.GetString("name")
			}
			return apiErr(409, fmt.Sprintf("%s already has a pass from %s", dup.GetString("holderName"), by))
		}
		pass, err = newPass(tx, "guest", "SENT", holderID, name, contact, u.Id)
		return err
	})
	if err != nil {
		return nil, err
	}

	claimURL := strings.TrimRight(appURL, "/") + "/claim/" + pass.GetString("claimToken")
	if isMail {
		ev := s.Settings().Event
		first := strings.Split(name, " ")[0]
		go s.Mail(email,
			fmt.Sprintf("%s sent you a pass to %s, IIM Ahmedabad", u.GetString("name"), ev.Title),
			fmt.Sprintf("પધારો, %s!\n\n%s has sent you a pass to %s at IIM Ahmedabad: %s, %s, from %s.\n\nClaim it in your name here: %s\n\nOr sign in with this email (Google or a one-time code) to see your pass.\n\nCarry a photo ID. The name must match your pass.\n\nCultcomm, IIM Ahmedabad",
				first, u.GetString("name"), ev.Title, ev.DateLabel, ev.Venue, ev.TimeLabel, claimURL))
	}
	me, err := s.Me(u)
	if err != nil {
		return nil, err
	}
	return &SendResult{Pass: s.passView(s.app, pass, false), ClaimURL: claimURL, Me: me}, nil
}

func (s *Service) ClaimLink(u *core.Record, passID, appURL string) (string, error) {
	p, err := s.app.FindFirstRecordByFilter("passes", "id = {:id} && issuer = {:u} && status != 'REVOKED'", dbx.Params{"id": passID, "u": u.Id})
	if err != nil {
		return "", apiErr(404, "Pass not found")
	}
	return strings.TrimRight(appURL, "/") + "/claim/" + p.GetString("claimToken"), nil
}

func (s *Service) RevokeSent(u *core.Record, passID string) error {
	return s.app.RunInTransaction(func(tx core.App) error {
		p, err := tx.FindFirstRecordByFilter("passes", "id = {:id} && issuer = {:u}", dbx.Params{"id": passID, "u": u.Id})
		if err != nil {
			return apiErr(404, "Pass not found")
		}
		if p.GetString("status") != "SENT" {
			return apiErr(409, "This pass has already been claimed, so it can't be taken back")
		}
		p.Set("status", "REVOKED")
		p.Set("revokedAt", nowMs())
		return tx.Save(p)
	})
}

// ---------- claim links (no sign-in needed) ----------

func (s *Service) claimRecord(token string) (*core.Record, error) {
	if len(token) < 10 {
		return nil, apiErr(404, "This pass link is no longer valid")
	}
	p, err := s.app.FindFirstRecordByFilter("passes", "claimToken = {:t}", dbx.Params{"t": token})
	if err != nil || p.GetString("status") == "REVOKED" {
		return nil, apiErr(404, "This pass link is no longer valid")
	}
	return p, nil
}

func (s *Service) ClaimInfo(token string) (*ClaimResponse, error) {
	p, err := s.claimRecord(token)
	if err != nil {
		return nil, err
	}
	claimed := p.GetString("status") == "CLAIMED"
	return &ClaimResponse{Event: s.Settings().Event, Pass: s.passView(s.app, p, claimed), Claimed: claimed}, nil
}

func (s *Service) Claim(token, nameRaw string) (*ClaimResponse, error) {
	p, err := s.claimRecord(token)
	if err != nil {
		return nil, err
	}
	if p.GetString("status") == "SENT" {
		name := cleanName(nameRaw)
		if len(name) < 2 || len(name) > 80 {
			return nil, apiErr(400, "Enter your name as on your ID")
		}
		p.Set("status", "CLAIMED")
		p.Set("claimedAt", nowMs())
		p.Set("holderName", name)
		if err := s.app.Save(p); err != nil {
			return nil, err
		}
	}
	return s.ClaimInfo(token)
}

// ---------- gate ----------

func (s *Service) EnteredCount() int {
	n, _ := s.app.CountRecords("passes", dbx.NewExp("enteredAt > 0"))
	return int(n)
}

func (s *Service) Scan(volunteer *core.Record, payloadRaw string, gateRaw int) (*ScanResult, error) {
	payload := strings.TrimSpace(payloadRaw)
	gate := min(max(gateRaw, 1), s.Settings().Event.Gates)

	var p *core.Record
	var err error
	switch {
	case strings.HasPrefix(payload, qrPrefix):
		p, err = s.app.FindFirstRecordByData("passes", "secret", strings.TrimPrefix(payload, qrPrefix))
	case codeRe.MatchString(payload):
		p, err = s.app.FindFirstRecordByData("passes", "code", strings.ToUpper(payload))
	default:
		err = errors.New("unknown payload")
	}
	res := func(outcome, title, sub, name, meta string) *ScanResult {
		return &ScanResult{Outcome: outcome, Title: title, Sub: sub, Name: name, Meta: meta, Entered: s.EnteredCount()}
	}
	if err != nil {
		if len(payload) > 40 {
			payload = payload[:40]
		}
		return res("invalid", "Not a valid pass", "This QR isn't a Garba Night pass. No entry.", "Unknown code", payload), nil
	}

	v := s.passView(s.app, p, false)
	var meta string
	switch v.Kind {
	case "guest":
		by := "a member"
		if v.IssuerName != nil {
			by = *v.IssuerName
		}
		meta = fmt.Sprintf("Guest of %s · %s", by, v.Code)
	case "exchange":
		college := "Pass exchange"
		if v.College != nil {
			college = *v.College
		}
		meta = fmt.Sprintf("%s · %s", college, v.Code)
	default:
		meta = fmt.Sprintf("%s · Own pass · %s", v.TypeLabel, v.Code)
	}
	switch v.Status {
	case "REVOKED":
		return res("revoked", "Pass taken back", "This pass was revoked. No entry.", v.HolderName, meta), nil
	case "SENT":
		return res("unclaimed", "Not claimed yet", "Ask them to open the link and claim the pass in their name", v.HolderName, meta), nil
	}

	var usedAt int64
	var usedGate int
	err = s.app.RunInTransaction(func(tx core.App) error {
		fresh, err := tx.FindRecordById("passes", p.Id)
		if err != nil {
			return err
		}
		if at := int64(fresh.GetFloat("enteredAt")); at > 0 {
			usedAt, usedGate = at, fresh.GetInt("enteredGate")
			return nil
		}
		fresh.Set("enteredAt", nowMs())
		fresh.Set("enteredGate", gate)
		fresh.Set("enteredBy", volunteer.Id)
		return tx.Save(fresh)
	})
	if err != nil {
		return nil, err
	}
	if usedAt > 0 {
		return res("used", "Already used", fmt.Sprintf("Scanned at Gate %d · %s", usedGate, istTime(usedAt)), v.HolderName, meta), nil
	}
	sub := "Welcome, khelaiya!"
	switch v.Kind {
	case "guest":
		sub = "Guest pass · check photo ID"
	case "exchange":
		sub = "Pass exchange guest · check college ID"
	}
	return res("allowed", "Entry allowed", sub, v.HolderName, meta), nil
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
