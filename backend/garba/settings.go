package garba

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"

	"github.com/pocketbase/pocketbase/core"
)

var GroupLabel = map[string]string{
	"pgp1": "PGP1", "student": "Student", "faculty": "Faculty & Staff", "exchange": "Exchange", "guest": "Guest",
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

// Limits are how many guests (friends or family) each group can add, on top of their own pass.
type Limits struct {
	PGP1    int `json:"pgp1"`
	Student int `json:"student"`
	Faculty int `json:"faculty"`
}

type Settings struct {
	Event  EventInfo `json:"event"`
	Limits Limits    `json:"limits"`
	// Email prefixes (before @) that mark PGP1 students, e.g. p26, f26.
	PGP1Prefixes []string `json:"pgp1Prefixes"`
}

// Placeholders from the design. Admins change these from the Settings tab.
func defaultSettings() Settings {
	return Settings{
		Event: EventInfo{
			Title:      "Garba Night",
			DateLabel:  "Fri 16 Oct",
			Venue:      "Football Ground",
			VenueShort: "Football Ground",
			TimeLabel:  "8 PM",
			DressCode:  "Dress code: chaniya choli, kediyu, kurta. Dandiyas provided.",
			Gates:      2,
		},
		Limits:       Limits{PGP1: 0, Student: 3, Faculty: 3},
		PGP1Prefixes: []string{"p26", "f26"},
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
		var saved SettingsInput
		if json.Unmarshal([]byte(rec.GetString("data")), &saved) == nil {
			out = merge(out, saved)
		}
	}
	s.cache.mu.Lock()
	s.cache.val = &out
	s.cache.mu.Unlock()
	return out
}

var prefixRe = regexp.MustCompile(`^[a-z0-9._-]{1,20}$`)

// SettingsInput is a partial update: anything left out keeps its current value.
type SettingsInput struct {
	Event  EventInfo `json:"event"`
	Limits struct {
		PGP1    *int `json:"pgp1"`
		Student *int `json:"student"`
		Faculty *int `json:"faculty"`
	} `json:"limits"`
	PGP1Prefixes []string `json:"pgp1Prefixes"`
}

func merge(dst Settings, src SettingsInput) Settings {
	set := func(d *string, v string) {
		if v = strings.TrimSpace(v); v != "" {
			if len(v) > 200 {
				v = v[:200]
			}
			*d = v
		}
	}
	set(&dst.Event.Title, src.Event.Title)
	set(&dst.Event.DateLabel, src.Event.DateLabel)
	set(&dst.Event.Venue, src.Event.Venue)
	set(&dst.Event.VenueShort, src.Event.VenueShort)
	set(&dst.Event.TimeLabel, src.Event.TimeLabel)
	set(&dst.Event.DressCode, src.Event.DressCode)
	if src.Event.Gates >= 1 && src.Event.Gates <= 20 {
		dst.Event.Gates = src.Event.Gates
	}
	lim := func(d *int, v *int) {
		if v != nil && *v >= 0 && *v <= 50 {
			*d = *v
		}
	}
	lim(&dst.Limits.PGP1, src.Limits.PGP1)
	lim(&dst.Limits.Student, src.Limits.Student)
	lim(&dst.Limits.Faculty, src.Limits.Faculty)
	if src.PGP1Prefixes != nil {
		var p []string
		for _, v := range src.PGP1Prefixes {
			v = strings.ToLower(strings.TrimSpace(v))
			if prefixRe.MatchString(v) {
				p = append(p, v)
			}
		}
		dst.PGP1Prefixes = p
	}
	return dst
}

func (s *Service) SaveSettings(in SettingsInput) (Settings, error) {
	next := merge(s.Settings(), in)
	rec, err := s.app.FindFirstRecordByFilter("garba_settings", "id != ''")
	if err != nil {
		col, err := s.app.FindCollectionByNameOrId("garba_settings")
		if err != nil {
			return next, err
		}
		rec = core.NewRecord(col)
	}
	rec.Set("data", next)
	if err := s.app.Save(rec); err != nil {
		return next, err
	}
	s.cache.mu.Lock()
	s.cache.val = &next
	s.cache.mu.Unlock()
	return next, nil
}
