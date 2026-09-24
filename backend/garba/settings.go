package garba

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/pocketbase/pocketbase/core"
)

var Groups = []string{"pgp1", "pgp2", "pgpx", "phd", "faculty", "staff", "exchange", "guest"}

var GroupLabel = map[string]string{
	"pgp1": "PGP1", "pgp2": "PGP2", "pgpx": "PGPX", "phd": "PhD", "faculty": "Faculty", "staff": "Staff", "exchange": "Exchange", "guest": "Guest",
}

type EventInfo struct {
	Title      string `json:"title"`
	DateLabel  string `json:"dateLabel"`
	Venue      string `json:"venue"`
	VenueShort string `json:"venueShort"`
	TimeLabel  string `json:"timeLabel"`
	DressCode  string `json:"dressCode"`
	Gates      int    `json:"gates"`
}

type Settings struct {
	Event  EventInfo      `json:"event"`
	Quotas map[string]int `json:"quotas"`
}

// Placeholders from the design. Admins change these from the Settings tab.
func defaultSettings() Settings {
	return Settings{
		Event: EventInfo{
			Title:      "Garba Night",
			DateLabel:  "Sat 17 Oct",
			Venue:      "Louis Kahn Plaza",
			VenueShort: "LKP",
			TimeLabel:  "8 PM",
			DressCode:  "Dress code: chaniya choli, kediyu, kurta. Dandiyas provided.",
			Gates:      2,
		},
		Quotas: map[string]int{"pgp1": 4, "pgp2": 4, "pgpx": 2, "phd": 2, "faculty": 4, "staff": 2, "exchange": 0, "guest": 0},
	}
}

type settingsCache struct {
	mu  sync.RWMutex
	val *Settings
}

func (s *Service) Settings() Settings {
	s.cache.mu.RLock()
	if v := s.cache.val; v != nil {
		s.cache.mu.RUnlock()
		return *v
	}
	s.cache.mu.RUnlock()

	out := defaultSettings()
	if rec, err := s.app.FindFirstRecordByFilter("garba_settings", "id != ''"); err == nil {
		var ev EventInfo
		if json.Unmarshal([]byte(rec.GetString("event")), &ev) == nil {
			mergeEvent(&out.Event, ev)
		}
		var q map[string]int
		if json.Unmarshal([]byte(rec.GetString("quotas")), &q) == nil {
			for k, v := range q {
				if _, ok := GroupLabel[k]; ok && v >= 0 && v <= 50 {
					out.Quotas[k] = v
				}
			}
		}
	}
	out.Quotas["guest"] = 0
	s.cache.mu.Lock()
	s.cache.val = &out
	s.cache.mu.Unlock()
	return out
}

func mergeEvent(dst *EventInfo, src EventInfo) {
	set := func(d *string, v string) {
		if v = strings.TrimSpace(v); v != "" {
			if len(v) > 200 {
				v = v[:200]
			}
			*d = v
		}
	}
	set(&dst.Title, src.Title)
	set(&dst.DateLabel, src.DateLabel)
	set(&dst.Venue, src.Venue)
	set(&dst.VenueShort, src.VenueShort)
	set(&dst.TimeLabel, src.TimeLabel)
	set(&dst.DressCode, src.DressCode)
	if src.Gates >= 1 && src.Gates <= 20 {
		dst.Gates = src.Gates
	}
}

func (s *Service) SaveSettings(in Settings) (Settings, error) {
	next := s.Settings()
	mergeEvent(&next.Event, in.Event)
	quotas := map[string]int{}
	for k, v := range next.Quotas {
		quotas[k] = v
	}
	for k, v := range in.Quotas {
		if _, ok := GroupLabel[k]; ok && k != "guest" && v >= 0 && v <= 50 {
			quotas[k] = v
		}
	}
	next.Quotas = quotas

	rec, err := s.app.FindFirstRecordByFilter("garba_settings", "id != ''")
	if err != nil {
		col, err := s.app.FindCollectionByNameOrId("garba_settings")
		if err != nil {
			return next, err
		}
		rec = core.NewRecord(col)
	}
	rec.Set("event", next.Event)
	rec.Set("quotas", next.Quotas)
	if err := s.app.Save(rec); err != nil {
		return next, err
	}
	s.cache.mu.Lock()
	s.cache.val = &next
	s.cache.mu.Unlock()
	return next, nil
}
