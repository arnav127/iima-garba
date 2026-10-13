package garba

import (
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Guests of pass holders.
//
// IIMA members add guests from their account (the guest pass's "issuer"). Some people who may
// bring family have no IIMA account: alumni are added through the exchange sheet and open their
// pass from a link. Their guests hang off their pass instead (the guest pass's "host").

// hostLimit is how many guests an exchange pass may add: the number set for that person
// (a "Guests" column in the sheet), else the alumni default if their college says alumni, else none.
func (s *Service) hostLimit(p *core.Record) int {
	if p == nil || p.GetString("kind") != "exchange" || p.GetString("status") != "ACTIVE" {
		return 0
	}
	if n := p.GetInt("guestLimit"); n > 0 {
		return n
	}
	if strings.Contains(strings.ToLower(p.GetString("college")), "alum") {
		return s.Settings().Limits.Alumni
	}
	return 0
}

// hostPass is the pass a signed-in non-member adds guests from (an alum who signs in with Google), or nil.
func (s *Service) hostPass(app core.App, u *core.Record) *core.Record {
	if isMemberGroup(group(u)) {
		return nil
	}
	recs, _ := app.FindRecordsByFilter("passes", "holder = {:u} && kind = 'exchange' && status = 'ACTIVE'", "seq", 0, 0, dbx.Params{"u": u.Id})
	for _, p := range recs {
		if s.hostLimit(p) > 0 {
			return p
		}
	}
	return nil
}

// guestOwner is who a new guest belongs to: a member (issuer) or a host pass (host).
type guestOwner struct {
	issuer, host string
	email        string // the owner's own address, which can't be added as a guest
	limit        int
}

func (o guestOwner) exp() dbx.HashExp {
	if o.host != "" {
		return dbx.HashExp{"host": o.host, "status": "ACTIVE"}
	}
	return dbx.HashExp{"issuer": o.issuer, "status": "ACTIVE"}
}

func cleanGuest(nameRaw, emailRaw string) (name, email string, err error) {
	name, email = cleanName(nameRaw), strings.ToLower(strings.TrimSpace(emailRaw))
	if len(name) < 2 || len(name) > 80 {
		return "", "", apiErr(400, "Enter their name as on their ID")
	}
	if email != "" && !isEmail(email) {
		return "", "", apiErr(400, "That email doesn't look right. Leave it empty to keep their pass on your phone.")
	}
	return name, email, nil
}

// addGuest creates a guest pass for owner. Run inside a transaction.
func (s *Service) addGuest(tx core.App, name, email string, o guestOwner) error {
	n, err := tx.CountRecords("passes", o.exp())
	if err != nil {
		return err
	}
	if int(n) >= o.limit {
		if o.limit == 0 {
			return apiErr(409, "Your pass is for yourself only")
		}
		return apiErr(409, "You've added all your guests")
	}
	holderID := ""
	if email != "" {
		if email == o.email {
			return apiErr(400, "That's your own email")
		}
		if s.isMemberEmail(email) {
			return apiErr(409, "IIMA members get their own pass when they sign in")
		}
		if dup, err := tx.FindFirstRecordByFilter("passes", "holderEmail = {:e} && status = 'ACTIVE'", dbx.Params{"e": email}); err == nil {
			by := "Cultcomm"
			if v := s.viewOne(tx, dup, false); v.IssuerName != nil {
				by = *v.IssuerName
			}
			return apiErr(409, fmt.Sprintf("%s already has a pass from %s", email, by))
		}
		if existing, err := tx.FindAuthRecordByEmail("users", email); err == nil {
			holderID = existing.Id
		}
	}
	extra := map[string]any{}
	if o.host != "" {
		extra["host"] = o.host
	}
	_, err = newPass(tx, "guest", holderID, name, email, o.issuer, extra)
	return err
}

func (s *Service) removeGuest(tx core.App, p *core.Record) error {
	if p.GetFloat("enteredAt") > 0 {
		return apiErr(409, "They've already entered, so this pass can't be removed")
	}
	p.Set("status", "REVOKED")
	p.Set("revokedAt", s.nowMs())
	return tx.Save(p)
}

// ---------- by pass link (no sign-in) ----------

func (s *Service) passByToken(app core.App, token string) (*core.Record, error) {
	if len(token) < 16 {
		return nil, apiErr(404, "This pass link isn't valid")
	}
	p, err := app.FindFirstRecordByData("passes", "linkToken", token)
	if err != nil || p.GetString("status") != "ACTIVE" {
		return nil, apiErr(404, "This pass link isn't valid any more")
	}
	return p, nil
}

func (s *Service) linkResponse(p *core.Record) (*LinkResponse, error) {
	res := &LinkResponse{Event: s.Settings().Event, Pass: s.viewOne(s.app, p, true), Guests: []PassView{}, ServerTime: s.nowMs()}
	if res.Limit = s.hostLimit(p); res.Limit > 0 {
		guests, err := s.app.FindRecordsByFilter("passes", "host = {:h} && status = 'ACTIVE'", "seq", 0, 0, dbx.Params{"h": p.Id})
		if err != nil {
			return nil, err
		}
		res.Guests = s.views(s.app, guests, true)
		res.Remaining = max(res.Limit-len(guests), 0)
	}
	return res, nil
}

// AddGuestByLink lets the holder of a pass link (an alum, say) add a family member.
func (s *Service) AddGuestByLink(token, nameRaw, emailRaw string) (*LinkResponse, error) {
	name, email, err := cleanGuest(nameRaw, emailRaw)
	if err != nil {
		return nil, err
	}
	var host *core.Record
	err = s.app.RunInTransaction(func(tx core.App) error {
		if host, err = s.passByToken(tx, token); err != nil {
			return err
		}
		return s.addGuest(tx, name, email, guestOwner{host: host.Id, email: host.GetString("holderEmail"), limit: s.hostLimit(host)})
	})
	if err != nil {
		return nil, err
	}
	return s.linkResponse(host)
}

func (s *Service) RemoveGuestByLink(token, id string) (*LinkResponse, error) {
	var host *core.Record
	err := s.app.RunInTransaction(func(tx core.App) (err error) {
		if host, err = s.passByToken(tx, token); err != nil {
			return err
		}
		p, err := tx.FindFirstRecordByFilter("passes", "id = {:id} && host = {:h} && status = 'ACTIVE'", dbx.Params{"id": id, "h": host.Id})
		if err != nil {
			return apiErr(404, "Pass not found")
		}
		return s.removeGuest(tx, p)
	})
	if err != nil {
		return nil, err
	}
	return s.linkResponse(host)
}
