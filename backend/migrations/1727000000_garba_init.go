package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Schema for Garba Night passes.
//
// Sign-in is Google (or Microsoft) OAuth2 only: no emails are ever sent.
// All reads and writes go through the custom /api/garba/* routes, so the collection
// API rules are locked down. The only open rule is list/view on passes for admins,
// which powers the realtime dashboard.
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		users.ListRule = types.Pointer("id = @request.auth.id")
		users.ViewRule = types.Pointer("id = @request.auth.id")
		users.CreateRule = nil
		users.UpdateRule = nil
		users.DeleteRule = nil
		users.PasswordAuth.Enabled = false
		users.OTP.Enabled = false
		users.OAuth2.Enabled = true
		users.AuthAlert.Enabled = false
		users.Fields.Add(
			// pgp1 | student | faculty come from the @iima.ac.in address; guest | exchange from the pass they hold.
			&core.SelectField{Name: "group", Values: []string{"pgp1", "student", "faculty", "exchange", "guest"}, MaxSelect: 1, Required: true},
			&core.BoolField{Name: "groupLocked"},
			&core.SelectField{Name: "role", Values: []string{"member", "volunteer", "admin"}, MaxSelect: 1, Required: true},
			// Per-person guest limit set by an admin; when hasLimit is false the group default applies.
			&core.BoolField{Name: "hasLimit"},
			&core.NumberField{Name: "guestLimit", OnlyInt: true},
		)
		if err := app.Save(users); err != nil {
			return err
		}

		passes := core.NewBaseCollection("passes")
		passes.ListRule = types.Pointer(`@request.auth.role = "admin"`)
		passes.ViewRule = passes.ListRule
		passes.Fields.Add(
			&core.NumberField{Name: "seq", OnlyInt: true, Required: true},
			&core.TextField{Name: "code", Required: true, Max: 20},
			// Key for the rotating QR. Only the holder, the person who added them, and the pass link see it.
			&core.TextField{Name: "secret", Required: true, Hidden: true, Max: 64},
			// Private link (/p/<token>) for guests without a Google account.
			&core.TextField{Name: "linkToken", Required: true, Hidden: true, Max: 64},
			&core.SelectField{Name: "kind", Values: []string{"own", "guest", "exchange"}, MaxSelect: 1, Required: true},
			&core.SelectField{Name: "status", Values: []string{"ACTIVE", "REVOKED"}, MaxSelect: 1, Required: true},
			&core.RelationField{Name: "holder", CollectionId: users.Id, MaxSelect: 1},
			&core.TextField{Name: "holderName", Required: true, Max: 120},
			&core.EmailField{Name: "holderEmail"},
			&core.TextField{Name: "college", Max: 120},
			&core.TextField{Name: "phone", Max: 30},
			&core.RelationField{Name: "issuer", CollectionId: users.Id, MaxSelect: 1, CascadeDelete: true},
			&core.NumberField{Name: "revokedAt", OnlyInt: true},
			&core.NumberField{Name: "enteredAt", OnlyInt: true},
			&core.NumberField{Name: "enteredGate", OnlyInt: true},
			&core.RelationField{Name: "enteredBy", CollectionId: users.Id, MaxSelect: 1},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		passes.AddIndex("idx_passes_seq", true, "seq", "")
		passes.AddIndex("idx_passes_code", true, "code", "")
		passes.AddIndex("idx_passes_link", true, "linkToken", "")
		passes.AddIndex("idx_passes_own", true, "holder", "kind = 'own' AND holder != ''")
		passes.AddIndex("idx_passes_issuer", false, "issuer", "")
		passes.AddIndex("idx_passes_holder", false, "holder", "")
		passes.AddIndex("idx_passes_email", false, "holderEmail", "")
		passes.AddIndex("idx_passes_entered", false, "enteredAt", "")
		if err := app.Save(passes); err != nil {
			return err
		}

		settings := core.NewBaseCollection("garba_settings")
		settings.Fields.Add(&core.JSONField{Name: "data", MaxSize: 20000})
		return app.Save(settings)
	}, func(app core.App) error {
		for _, name := range []string{"garba_settings", "passes"} {
			if c, err := app.FindCollectionByNameOrId(name); err == nil {
				if err := app.Delete(c); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
