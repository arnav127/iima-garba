package garba

import (
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Printed passes: paper passes for people without a smartphone (support staff, for example).
// Paper can't show a rotating code, so these carry a fixed QR: G3.<passId>.<sig>, where
// sig = HMAC-SHA256(secret, "<passId>.print") truncated. The gate accepts a fixed code only for
// passes of kind "print", so a phone pass still can't be screenshotted, and like every pass a
// printed one gets in exactly once (a photocopy of a used pass shows "Already used").
const (
	printPrefix   = "G3."
	MaxPrintBatch = 400
)

func printPayload(secret, passID string) string {
	mac := hmacSum(secret, passID+".print")
	return printPrefix + passID + "." + mac
}

// PrintPass is one pass on a printed sheet.
type PrintPass struct {
	Name      string `json:"name"`
	Code      string `json:"code"`
	QR        string `json:"qr"`
	Status    string `json:"status"`
	EnteredAt *int64 `json:"enteredAt"`
}

// PrintBatch is one print run: everything needed to lay out the sheets (again).
type PrintBatch struct {
	Batch  string      `json:"batch"`
	Label  string      `json:"label"`
	Event  EventInfo   `json:"event"`
	Passes []PrintPass `json:"passes"`
}

// PrintBatchInfo is a line in the list of earlier print runs.
type PrintBatchInfo struct {
	Batch     string `db:"batch" json:"batch"`
	Label     string `db:"label" json:"label"`
	Count     int    `db:"count" json:"count"`
	Entered   int    `db:"entered" json:"entered"`
	Cancelled int    `db:"cancelled" json:"cancelled"`
}

func (s *Service) printBatch(batch string) (*PrintBatch, error) {
	recs, err := s.app.FindRecordsByFilter("passes", "kind = 'print' && batch = {:b}", "seq", 0, 0, dbx.Params{"b": batch})
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, apiErr(404, "Print run not found")
	}
	out := &PrintBatch{Batch: batch, Label: recs[0].GetString("college"), Event: s.Settings().Event, Passes: make([]PrintPass, 0, len(recs))}
	for _, p := range recs {
		pp := PrintPass{Name: p.GetString("holderName"), Code: p.GetString("code"), QR: printPayload(p.GetString("secret"), p.Id), Status: p.GetString("status")}
		if at := int64(p.GetFloat("enteredAt")); at > 0 {
			pp.EnteredAt = &at
		}
		out.Passes = append(out.Passes, pp)
	}
	return out, nil
}

// PrintBatch returns an earlier print run, to print the same passes again.
func (s *Service) PrintBatch(batch string) (*PrintBatch, error) {
	return s.printBatch(strings.TrimSpace(batch))
}

// GeneratePrintPasses creates count new printed passes labelled e.g. "Support staff 041".
func (s *Service) GeneratePrintPasses(count int, labelRaw string) (*PrintBatch, error) {
	label := cleanName(labelRaw)
	if label == "" {
		label = "Support staff"
	}
	if len(label) > 40 {
		return nil, apiErr(400, "Keep the label under 40 characters")
	}
	if count < 1 || count > MaxPrintBatch {
		return nil, apiErr(400, fmt.Sprintf("Generate between 1 and %d passes at a time", MaxPrintBatch))
	}
	// Named by its pass numbers, which never repeat: "Support staff 041–080 · 13 Oct 6:42 PM".
	var batch string
	err := s.app.RunInTransaction(func(tx core.App) error {
		n, err := tx.CountRecords("passes", dbx.HashExp{"kind": "print"})
		if err != nil {
			return err
		}
		first, last := int(n)+1, int(n)+count
		batch = fmt.Sprintf("%s %03d–%03d · %s", label, first, last, s.Now().In(ist).Format("2 Jan 3:04 PM"))
		for i := first; i <= last; i++ {
			if _, err := newPass(tx, "print", "", fmt.Sprintf("%s %03d", label, i), "", "", map[string]any{"college": label, "batch": batch}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.printBatch(batch)
}

// PrintBatches lists earlier print runs, newest first.
func (s *Service) PrintBatches() ([]PrintBatchInfo, error) {
	out := []PrintBatchInfo{}
	err := s.app.DB().NewQuery(`SELECT batch, MAX(college) AS label, COUNT(*) AS count,
		COALESCE(SUM(enteredAt > 0), 0) AS entered, COALESCE(SUM(status = 'REVOKED'), 0) AS cancelled
		FROM passes WHERE kind = 'print' GROUP BY batch ORDER BY MIN(seq) DESC`).All(&out)
	return out, err
}

// scanPrinted checks a fixed QR from a paper pass.
func (s *Service) scanPrinted(volunteer *core.Record, payload string, gate int) (*ScanResult, error) {
	invalid := s.result("invalid", "Not a valid pass", "This QR isn't a Garba Night pass. No entry.", nil)
	parts := strings.Split(strings.TrimPrefix(payload, printPrefix), ".")
	if len(parts) != 2 {
		return invalid, nil
	}
	p, err := s.app.FindRecordById("passes", parts[0])
	// Only printed passes have a fixed code: a phone pass must show its live, rotating one.
	if err != nil || p.GetString("kind") != "print" || !hmacEqual(parts[1], hmacSum(p.GetString("secret"), p.Id+".print")) {
		return invalid, nil
	}
	return s.admit(volunteer, p, s.viewOne(s.app, p, false), gate)
}
