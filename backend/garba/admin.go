package garba

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/xuri/excelize/v2"
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
	Kind string `db:"kind" json:"kind"`
	Gate int    `db:"gate" json:"gate"`
	At   int64  `db:"at" json:"at"`
}

type AdminStats struct {
	Entered  int           `json:"entered"`
	Issued   int           `json:"issued"`
	Members  int           `json:"members"`
	Guests   int           `json:"guests"`
	Exchange int           `json:"exchange"`
	ByGate   []GateCount   `json:"byGate"`
	ByKind   []KindCount   `json:"byKind"`
	Recent   []RecentEntry `json:"recent"`
}

func (s *Service) Stats() (*AdminStats, error) {
	db := s.app.DB()
	var c struct {
		Entered  int `db:"entered"`
		Issued   int `db:"issued"`
		Members  int `db:"members"`
		Guests   int `db:"guests"`
		Exchange int `db:"exchange"`
	}
	err := db.NewQuery(`SELECT
		(SELECT COUNT(*) FROM passes WHERE enteredAt > 0) AS entered,
		(SELECT COUNT(*) FROM passes WHERE status = 'ACTIVE') AS issued,
		(SELECT COUNT(*) FROM passes WHERE status = 'ACTIVE' AND kind = 'own') AS members,
		(SELECT COUNT(*) FROM passes WHERE status = 'ACTIVE' AND kind = 'guest') AS guests,
		(SELECT COUNT(*) FROM passes WHERE status = 'ACTIVE' AND kind = 'exchange') AS exchange`).One(&c)
	if err != nil {
		return nil, err
	}
	out := &AdminStats{Entered: c.Entered, Issued: c.Issued, Members: c.Members, Guests: c.Guests, Exchange: c.Exchange,
		ByGate: []GateCount{}, ByKind: []KindCount{}, Recent: []RecentEntry{}}
	if err := db.NewQuery("SELECT enteredGate AS gate, COUNT(*) AS count FROM passes WHERE enteredAt > 0 GROUP BY enteredGate ORDER BY enteredGate").All(&out.ByGate); err != nil {
		return nil, err
	}
	if err := db.NewQuery("SELECT kind, COUNT(*) AS issued, SUM(enteredAt > 0) AS entered FROM passes WHERE status = 'ACTIVE' GROUP BY kind ORDER BY kind").All(&out.ByKind); err != nil {
		return nil, err
	}
	if err := db.NewQuery("SELECT holderName AS name, code, kind, enteredGate AS gate, CAST(enteredAt AS INTEGER) AS at FROM passes WHERE enteredAt > 0 ORDER BY enteredAt DESC LIMIT 8").All(&out.Recent); err != nil {
		return nil, err
	}
	return out, nil
}

var passFilters = map[string]string{
	"entered":  " && enteredAt > 0",
	"waiting":  " && enteredAt = 0 && status = 'ACTIVE'",
	"own":      " && kind = 'own'",
	"guest":    " && kind = 'guest'",
	"exchange": " && kind = 'exchange'",
	"revoked":  " && status = 'REVOKED'",
}

func (s *Service) AdminPasses(q, filter string) ([]PassView, error) {
	f := "(holderName ~ {:q} || code ~ {:q} || holderEmail ~ {:q} || college ~ {:q} || issuer.name ~ {:q} || holder.email ~ {:q})" + passFilters[filter]
	recs, err := s.app.FindRecordsByFilter("passes", f, "-enteredAt,-seq", 200, 0, dbx.Params{"q": strings.TrimSpace(q)})
	if err != nil {
		return nil, err
	}
	return s.views(s.app, recs, false), nil
}

type AdminPerson struct {
	Me
	Guests int `json:"guests"`
	Limit  int `json:"limit"`
}

func (s *Service) guestCounts() (map[string]int, error) {
	var counts []struct {
		Issuer string `db:"issuer"`
		N      int    `db:"n"`
	}
	if err := s.app.DB().NewQuery("SELECT issuer, COUNT(*) AS n FROM passes WHERE issuer != '' AND status = 'ACTIVE' GROUP BY issuer").All(&counts); err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, c := range counts {
		out[c.Issuer] = c.N
	}
	return out, nil
}

func (s *Service) person(u *core.Record, counts map[string]int) AdminPerson {
	return AdminPerson{Me: toMe(u), Guests: counts[u.Id], Limit: s.GuestLimit(u)}
}

func (s *Service) AdminPeople(q string) ([]AdminPerson, error) {
	recs, err := s.app.FindRecordsByFilter("users", "name ~ {:q} || email ~ {:q}", "name", 200, 0, dbx.Params{"q": strings.TrimSpace(q)})
	if err != nil {
		return nil, err
	}
	counts, err := s.guestCounts()
	if err != nil {
		return nil, err
	}
	out := make([]AdminPerson, 0, len(recs))
	for _, u := range recs {
		out = append(out, s.person(u, counts))
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

func (s *Service) updatePass(id string, fn func(p *core.Record) error) error {
	return s.app.RunInTransaction(func(tx core.App) error {
		p, err := s.passByID(tx, id)
		if err != nil {
			return err
		}
		if err := fn(p); err != nil {
			return err
		}
		return tx.Save(p)
	})
}

func (s *Service) AdminRevoke(id string) error {
	return s.updatePass(id, func(p *core.Record) error {
		if p.GetString("status") == "REVOKED" || p.GetFloat("enteredAt") > 0 {
			return apiErr(409, "This pass can't be cancelled (already cancelled or already entered)")
		}
		p.Set("status", "REVOKED")
		p.Set("revokedAt", s.nowMs())
		return nil
	})
}

func (s *Service) AdminRestore(id string) error {
	return s.updatePass(id, func(p *core.Record) error {
		if p.GetString("status") != "REVOKED" {
			return apiErr(409, "Pass is not cancelled")
		}
		p.Set("status", "ACTIVE")
		p.Set("revokedAt", 0)
		return nil
	})
}

func (s *Service) AdminUndoEntry(id string) error {
	return s.updatePass(id, func(p *core.Record) error {
		if p.GetFloat("enteredAt") == 0 {
			return apiErr(409, "This pass has not been scanned in")
		}
		p.Set("enteredAt", 0)
		p.Set("enteredGate", 0)
		p.Set("enteredBy", "")
		return nil
	})
}

// UpdatePerson changes a person's role, cohort or guest limit. Nil fields are left alone;
// a negative limit clears the override so the group default applies again.
type PersonUpdate struct {
	Role  *string `json:"role"`
	Group *string `json:"group"`
	Limit *int    `json:"limit"`
}

func (s *Service) UpdatePerson(id string, in PersonUpdate) (*AdminPerson, error) {
	u, err := s.app.FindRecordById("users", id)
	if err != nil {
		return nil, apiErr(404, "Person not found")
	}
	if in.Role != nil {
		if !slices.Contains([]string{"member", "volunteer", "admin"}, *in.Role) {
			return nil, apiErr(400, "Unknown role")
		}
		u.Set("role", *in.Role)
	}
	if in.Group != nil {
		if !isMemberGroup(*in.Group) || !isMemberGroup(group(u)) {
			return nil, apiErr(400, "Only IIMA members can move between PGP1, Student and Faculty & Staff")
		}
		u.Set("group", *in.Group)
		u.Set("groupLocked", true)
	}
	if in.Limit != nil {
		switch {
		case *in.Limit < 0:
			u.Set("hasLimit", false)
			u.Set("guestLimit", 0)
		case *in.Limit <= 50:
			u.Set("hasLimit", true)
			u.Set("guestLimit", *in.Limit)
		default:
			return nil, apiErr(400, "Limit must be 0–50")
		}
	}
	if err := s.app.Save(u); err != nil {
		return nil, err
	}
	counts, err := s.guestCounts()
	if err != nil {
		return nil, err
	}
	p := s.person(u, counts)
	return &p, nil
}

// GrantAccess gives a role to someone by email, creating their account if they haven't signed in yet
// (IIMA addresses only, so volunteers can be set up before the night).
func (s *Service) GrantAccess(emailRaw, r string) (*AdminPerson, error) {
	email := strings.ToLower(strings.TrimSpace(emailRaw))
	if !isEmail(email) {
		return nil, apiErr(400, "Enter a valid email")
	}
	if !slices.Contains([]string{"member", "volunteer", "admin"}, r) {
		return nil, apiErr(400, "Unknown role")
	}
	u, err := s.app.FindAuthRecordByEmail("users", email)
	if err != nil {
		if !s.isMemberEmail(email) {
			return nil, apiErr(404, "They haven't signed in yet. Non-IIMA people need to sign in once before getting a role.")
		}
		if u, err = s.SignInUser(email, ""); err != nil {
			return nil, err
		}
	}
	return s.UpdatePerson(u.Id, PersonUpdate{Role: &r})
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

// readRows returns the rows of the first sheet of an .xlsx file, or of a .csv file.
func readRows(filename string, data []byte) ([][]string, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".xlsx", ".xlsm":
		f, err := excelize.OpenReader(bytes.NewReader(data))
		if err != nil {
			return nil, apiErr(400, "Couldn't open the Excel file")
		}
		defer f.Close()
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, apiErr(400, "The Excel file has no sheets")
		}
		return f.GetRows(sheets[0])
	case ".csv", ".txt":
		r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
		r.FieldsPerRecord = -1
		r.TrimLeadingSpace = true
		return r.ReadAll()
	}
	return nil, apiErr(400, "Upload an .xlsx or .csv file")
}

// ImportExchange adds exchange guests from a sheet with columns Name, Email, College (and optionally Phone).
// Each gets an exchange pass. Rows are matched on email (or name + college) so re-uploading updates them.
func (s *Service) ImportExchange(filename string, data []byte) (*ImportResult, error) {
	rows, err := readRows(filename, data)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, apiErr(400, "The file is empty")
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		h = headerClean.ReplaceAllString(strings.ToLower(h), "")
		for key, aliases := range map[string][]string{
			"name":    {"name", "fullname", "guestname", "studentname"},
			"email":   {"email", "emailid", "emailaddress", "mail", "gmail"},
			"college": {"college", "institute", "institution", "university", "school"},
			"phone":   {"phone", "mobile", "phonenumber", "mobilenumber", "contact", "whatsapp"},
		} {
			if _, taken := col[key]; !taken && slices.Contains(aliases, h) {
				col[key] = i
			}
		}
	}
	if _, ok := col["name"]; !ok {
		return nil, apiErr(400, "The first row needs column headings: Name, Email, College (Phone optional)")
	}
	get := func(row []string, key string) string {
		if i, ok := col[key]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	out := &ImportResult{Errors: []ImportError{}}
	err = s.app.RunInTransaction(func(tx core.App) error {
		for i, row := range rows[1:] {
			line := i + 2
			name, email := cleanName(get(row, "name")), strings.ToLower(get(row, "email"))
			college, phone := get(row, "college"), get(row, "phone")
			if name == "" && email == "" {
				continue
			}
			switch {
			case len(name) < 2:
				out.Errors = append(out.Errors, ImportError{line, "name is missing"})
				continue
			case email != "" && !isEmail(email):
				out.Errors = append(out.Errors, ImportError{line, fmt.Sprintf("bad email %q", email)})
				continue
			case s.isMemberEmail(email):
				out.Errors = append(out.Errors, ImportError{line, "IIMA addresses get their own pass by signing in"})
				continue
			}
			var existing *core.Record
			if email != "" {
				existing, _ = tx.FindFirstRecordByFilter("passes", "holderEmail = {:e} && kind != 'own'", dbx.Params{"e": email})
			} else {
				existing, _ = tx.FindFirstRecordByFilter("passes", "kind = 'exchange' && holderName = {:n} && college = {:c}", dbx.Params{"n": name, "c": college})
			}
			if existing != nil {
				if existing.GetString("kind") != "exchange" {
					out.Errors = append(out.Errors, ImportError{line, email + " already has a guest pass"})
					continue
				}
				existing.Set("holderName", name)
				existing.Set("college", college)
				existing.Set("phone", phone)
				existing.Set("status", "ACTIVE")
				if err := tx.Save(existing); err != nil {
					return err
				}
				out.Updated++
				continue
			}
			holderID := ""
			if email != "" {
				if u, err := tx.FindAuthRecordByEmail("users", email); err == nil {
					holderID = u.Id
				}
			}
			if _, err := newPass(tx, "exchange", holderID, name, email, "", map[string]any{"college": college, "phone": phone}); err != nil {
				return err
			}
			out.Created++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func csvBytes(header []string, rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xef\xbb\xbf") // BOM so Excel opens UTF-8 correctly
	w := csv.NewWriter(&buf)
	w.Write(header)
	w.WriteAll(rows)
	return buf.Bytes(), w.Error()
}

// ExchangeLinks lists every exchange guest with their private pass link, to share on WhatsApp
// or send to the partner college (the app never sends email).
func (s *Service) ExchangeLinks(appURL string) ([]byte, error) {
	recs, err := s.app.FindRecordsByFilter("passes", "kind = 'exchange' && status = 'ACTIVE'", "college,holderName", 0, 0)
	if err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(recs))
	for _, p := range recs {
		rows = append(rows, []string{p.GetString("holderName"), p.GetString("holderEmail"), p.GetString("phone"), p.GetString("college"),
			p.GetString("code"), strings.TrimRight(appURL, "/") + "/p/" + p.GetString("linkToken")})
	}
	return csvBytes([]string{"name", "email", "phone", "college", "code", "pass_link"}, rows)
}

func (s *Service) ExportCSV() ([]byte, error) {
	recs, err := s.app.FindRecordsByFilter("passes", "id != ''", "seq", 0, 0)
	if err != nil {
		return nil, err
	}
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	rows := [][]string{}
	for _, v := range s.views(s.app, recs, false) {
		entered, gate := "", ""
		if v.EnteredAt != nil {
			entered = time.UnixMilli(*v.EnteredAt).In(ist).Format("2006-01-02 15:04:05")
			gate = fmt.Sprint(*v.EnteredGate)
		}
		rows = append(rows, []string{v.Code, v.Kind, v.Status, v.HolderName, deref(v.HolderEmail), v.TypeLabel, deref(v.College), deref(v.IssuerName), entered, gate})
	}
	return csvBytes([]string{"code", "kind", "status", "name", "email", "type", "college", "added_by", "entered_at_ist", "gate"}, rows)
}
