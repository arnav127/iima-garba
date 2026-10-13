package migrations

import (
	"slices"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Printed passes and guests of pass holders.
//
//   - kind "print": paper passes for people without a smartphone, generated in batches.
//   - batch: which print run a printed pass belongs to (for printing the same sheet again).
//   - guestLimit: guests an exchange pass holder may add (0 = the default for their college).
//   - host: the pass a guest was added from, for hosts without an IIMA account (alumni using a pass link).
func init() {
	m.Register(func(app core.App) error {
		passes, err := app.FindCollectionByNameOrId("passes")
		if err != nil {
			return err
		}
		if kind, ok := passes.Fields.GetByName("kind").(*core.SelectField); ok && !slices.Contains(kind.Values, "print") {
			kind.Values = append(kind.Values, "print")
		}
		passes.Fields.Add(
			&core.TextField{Name: "batch", Max: 80},
			&core.NumberField{Name: "guestLimit", OnlyInt: true},
			&core.RelationField{Name: "host", CollectionId: passes.Id, MaxSelect: 1},
		)
		passes.AddIndex("idx_passes_host", false, "host", "")
		passes.AddIndex("idx_passes_batch", false, "batch", "")
		return app.Save(passes)
	}, func(app core.App) error {
		passes, err := app.FindCollectionByNameOrId("passes")
		if err != nil {
			return err
		}
		passes.RemoveIndex("idx_passes_host")
		passes.RemoveIndex("idx_passes_batch")
		for _, f := range []string{"batch", "guestLimit", "host"} {
			passes.Fields.RemoveByName(f)
		}
		return app.Save(passes)
	})
}
