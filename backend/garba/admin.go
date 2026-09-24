package garba

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type GateCount struct {
	Gate  int `db:"gate" json:"gate"`
	Count int `db:"count" json:"count"`
}

type KindCount struct {
	Kind    string `db:"kind" json:"kind"`
	Issued  int    `db:"issued" json:"issued"`
	Entered int    `db:"entered" json:"entered"`
}

type RecentEntry struct {
	Name string `db:"name" json:"name"`
	Code string `db:"code" json:"code"`
	Gate int    `db:"gate" json:"gate"`
	At   int64  `db:"at" json:"at"`
}

type AdminStats struct {
	Entered       int           `json:"entered"`
	Issued        int           `json:"issued"`
	Claimed       int           `json:"claimed"`
	PendingClaims int           `json:"pendingClaims"`
	Members       int           `json:"members"`
	Exchange      int           `json:"exchange"`
	ByGate        []GateCount   `json:"byGate"`
	ByKind        []KindCount   `json:"byKind"`
	Recent        []RecentEntry `json:"recent"`
}

func (s *Service) Stats() (*AdminStats, error) {
	db := s.app.DB()
	var c struct {
		Entered  int `db:"entered"`
		Issued   int `db:"issued"`
		Claimed  int `db:"claimed"`
		Pending  int `db:"pending"`
		Members  int `db:"members"`
		Exchange int `db:"exchange"`
	}
	err := db.NewQuery(`SELECT
		(SELECT COUNT(*) FROM passes WHERE enteredAt > 0) AS entered,
		(SELECT COUNT(*) FROM passes WHERE status != 'REVOKED') AS issued,
		(SELECT COUNT(*) FROM passes WHERE status = 'CLAIMED') AS claimed,
		(SELECT COUNT(*) FROM passes WHERE status = 'SENT') AS pending,
		(SELECT COUNT(*) FROM users WHERE [[group]] NOT IN ('exchange', 'guest')) AS members,
		(SELECT COUNT(*) FROM users WHERE [[group]] = 'exchange') AS exchange`).One(&c)
	if err != nil {
		return nil, err
	}
	out := &AdminStats{Entered: c.Entered, Issued: c.Issued, Claimed: c.Claimed, PendingClaims: c.Pending, Members: c.Members, Exchange: c.Exchange,
		ByGate: []GateCount{}, ByKind: []KindCount{}, Recent: []RecentEntry{}}
	if err := db.NewQuery("SELECT enteredGate AS gate, COUNT(*) AS count FROM passes WHERE enteredAt > 0 GROUP BY enteredGate ORDER BY enteredGate").All(&out.ByGate); err != nil {
		return nil, err
	}
	if err := db.NewQuery("SELECT kind, COUNT(*) AS issued, SUM(enteredAt > 0) AS entered FROM passes WHERE status != 'REVOKED' GROUP BY kind ORDER BY kind").All(&out.ByKind); err != nil {
		return nil, err
	}
	if err := db.NewQuery("SELECT holderName AS name, code, enteredGate AS gate, CAST(enteredAt AS INTEGER) AS at FROM passes WHERE enteredAt > 0 ORDER BY enteredAt DESC LIMIT 8").All(&out.Recent); err != nil {
		return nil, err
	}
	return out, nil
}

var passFilters = map[string]string{
	"entered":   " && enteredAt > 0",
	"waiting":   " && enteredAt = 0 && status = 'CLAIMED'",
	"unclaimed": " && status = 'SENT'",
	"guest":     " && kind = 'guest'",
	"exchange":  " && kind = 'exchange'",
	"revoked":   " && status = 'REVOKED'",
}

func (s *Service) AdminPasses(q, filter string) ([]PassView, error) {
	f := "(holderName ~ {:q} || code ~ {:q} || holderContact ~ {:q} || issuer.name ~ {:q})" + passFilters[filter]
	recs, err := s.app.FindRecordsByFilter("passes", f, "-enteredAt,-seq", 200, 0, dbx.Params{"q": strings.TrimSpace(q)})
	if err != nil {
		return nil, err
	}
	return s.views(s.app, recs, false), nil
}

type AdminPerson struct {
	ID      string  `json:"id"`
	Email   string  `json:"email"`
	Name    string  `json:"name"`
	Group   string  `json:"group"`
	Role    string  `json:"role"`
	College *string `json:"college"`
	Sent    int     `json:"sent"`
}

func (s *Service) AdminPeople(q string) ([]AdminPerson, error) {
	recs, err := s.app.FindRecordsByFilter("users", "name ~ {:q} || email ~ {:q} || college ~ {:q}", "name", 200, 0, dbx.Params{"q": strings.TrimSpace(q)})
	if err != nil {
		return nil, err
	}
	var counts []struct {
		Issuer string `db:"issuer"`
		N      int    `db:"n"`
	}
	if err := s.app.DB().NewQuery("SELECT issuer, COUNT(*) AS n FROM passes WHERE issuer != '' AND status != 'REVOKED' GROUP BY issuer").All(&counts); err != nil {
		return nil, err
	}
	sent := map[string]int{}
	for _, c := range counts {
		sent[c.Issuer] = c.N
	}
	out := make([]AdminPerson, 0, len(recs))
	for _, u := range recs {
		m := toMe(u)
		out = append(out, AdminPerson{ID: m.ID, Email: m.Email, Name: m.Name, Group: m.Group, Role: m.Role, College: m.College, Sent: sent[u.Id]})
	}
	return out, nil
}

func (s *Service) passByID(tx core.App, id string) (*core.Record, error) {
	p, err := tx.FindRecordById("passes", id)
	if err != nil {
		return nil, apiErr(404, "Pass not found")
	}
	return p, nil
}

func (s *Service) AdminRevoke(id string) error {
	return s.app.RunInTransaction(func(tx core.App) error {
		p, err := s.passByID(tx, id)
		if err != nil {
			return err
		}
		if p.GetString("status") == "REVOKED" || p.GetFloat("enteredAt") > 0 {
			return apiErr(409, "This pass can't be revoked (already revoked or already entered)")
		}
		p.Set("status", "REVOKED")
		p.Set("revokedAt", nowMs())
		return tx.Save(p)
	})
}

func (s *Service) AdminRestore(id string) error {
	return s.app.RunInTransaction(func(tx core.App) error {
		p, err := s.passByID(tx, id)
		if err != nil {
			return err
		}
		if p.GetString("status") != "REVOKED" {
			return apiErr(409, "Pass is not revoked")
		}
		status := "SENT"
		if p.GetFloat("claimedAt") > 0 {
			status = "CLAIMED"
		}
		p.Set("status", status)
		p.Set("revokedAt", 0)
		return tx.Save(p)
	})
}

func (s *Service) AdminUndoEntry(id string) error {
	return s.app.RunInTransaction(func(tx core.App) error {
		p, err := s.passByID(tx, id)
		if err != nil {
			return err
		}
		if p.GetFloat("enteredAt") == 0 {
			return apiErr(409, "This pass has not been scanned in")
		}
		p.Set("enteredAt", 0)
		p.Set("enteredGate", 0)
		p.Set("enteredBy", "")
		return tx.Save(p)
	})
}

func (s *Service) AdminSetRole(id, r string) error {
	if !slices.Contains([]string{"member", "volunteer", "admin"}, r) {
		return apiErr(400, "Unknown role")
	}
	u, err := s.app.FindRecordById("users", id)
	if err != nil {
		return apiErr(404, "Person not found")
	}
	u.Set("role", r)
	return s.app.Save(u)
}

type ImportError struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

type ImportResult struct {
	Created int           `json:"created"`
	Updated int           `json:"updated"`
	Errors  []ImportError `json:"errors"`
}

var headerClean = regexp.MustCompile(`[^a-z]`)

// ImportPeople upserts people from CSV with header: email,name,group[,college][,role]
// group is one of pgp1, pgp2, pgpx, phd, faculty, staff, exchange. Exchange rows need a college.
// Everyone gets their own entry pass.
func (s *Service) ImportPeople(text string) (*ImportResult, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(text, "\uFEFF")))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		return nil, apiErr(400, "The CSV is empty")
	}
	col := map[string]int{}
	for i, h := range header {
		col[headerClean.ReplaceAllString(strings.ToLower(h), "")] = i
	}
	if _, ok := col["email"]; !ok {
		return nil, apiErr(400, "The CSV needs a header row with at least: email,name,group")
	}
	get := func(row []string, keys ...string) string {
		for _, k := range keys {
			if i, ok := col[k]; ok && i < len(row) {
				return strings.TrimSpace(row[i])
			}
		}
		return ""
	}

	out := &ImportResult{Errors: []ImportError{}}
	err = s.app.RunInTransaction(func(tx core.App) error {
		for line := 2; ; line++ {
			row, err := r.Read()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				out.Errors = append(out.Errors, ImportError{line, "could not read this row"})
				continue
			}
			email := strings.ToLower(get(row, "email"))
			name := cleanName(get(row, "name"))
			grp := strings.ToLower(strings.ReplaceAll(get(row, "group", "grp", "batch"), " ", ""))
			college := get(row, "college")
			rl := strings.ToLower(get(row, "role"))
			switch {
			case email == "" && name == "" && grp == "":
				continue
			case !isEmail(email):
				out.Errors = append(out.Errors, ImportError{line, fmt.Sprintf("bad email %q", email)})
				continue
			case GroupLabel[grp] == "" || grp == "guest":
				out.Errors = append(out.Errors, ImportError{line, fmt.Sprintf("unknown group %q", grp)})
				continue
			case grp == "exchange" && college == "":
				out.Errors = append(out.Errors, ImportError{line, "exchange guests need a college"})
				continue
			case rl != "" && !slices.Contains([]string{"member", "volunteer", "admin"}, rl):
				out.Errors = append(out.Errors, ImportError{line, fmt.Sprintf("unknown role %q", rl)})
				continue
			}
			u, err := tx.FindAuthRecordByEmail("users", email)
			if err != nil {
				if name == "" {
					out.Errors = append(out.Errors, ImportError{line, "name is missing"})
					continue
				}
				if rl == "" {
					rl = "member"
				}
				if u, err = newUser(tx, email, name, grp, rl, college); err != nil {
					return err
				}
				out.Created++
			} else {
				if name != "" {
					u.Set("name", name)
				}
				u.Set("group", grp)
				u.Set("college", college)
				if rl != "" {
					u.Set("role", rl)
				}
				if err := tx.Save(u); err != nil {
					return err
				}
				out.Updated++
			}
			if err := ensureOwnPass(tx, u); err != nil {
				return err
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) ExportCSV() ([]byte, error) {
	recs, err := s.app.FindRecordsByFilter("passes", "id != ''", "seq", 0, 0)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("\uFEFF")
	w := csv.NewWriter(&buf)
	w.Write([]string{"code", "kind", "status", "holder_name", "holder_contact", "holder_email", "type", "college", "issued_by", "entered_at_ist", "gate"})
	for _, v := range s.views(s.app, recs, false) {
		entered, gate := "", ""
		if v.EnteredAt != nil {
			entered = time.UnixMilli(*v.EnteredAt).In(ist).Format("2006-01-02 15:04:05")
			gate = fmt.Sprint(*v.EnteredGate)
		}
		deref := func(p *string) string {
			if p == nil {
				return ""
			}
			return *p
		}
		w.Write([]string{v.Code, v.Kind, v.Status, v.HolderName, v.HolderContact, deref(v.HolderEmail), v.TypeLabel, deref(v.College), deref(v.IssuerName), entered, gate})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}
