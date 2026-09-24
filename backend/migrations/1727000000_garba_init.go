package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Schema for Garba Night passes.
//
// All reads and writes go through the custom /api/garba/* routes, so the collection
// API rules are locked down. The only open rule is list/view on passes for admins,
// which powers the realtime dashboard. (Volunteers poll the entry count instead, so
// they never see other people's contact details.)
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
		users.OTP.Enabled = true
		users.OTP.Duration = 600
		users.OTP.Length = 6
		users.OTP.EmailTemplate.Subject = "{OTP} is your Garba Night sign-in code"
		users.OTP.EmailTemplate.Body = `<p>કેમ છો!</p><p>Your Garba Night sign-in code is <strong style="font-size:22px;letter-spacing:4px">{OTP}</strong>. It expires in 10 minutes.</p><p>Cultcomm, IIM Ahmedabad</p>`
		users.OAuth2.Enabled = true
		users.AuthAlert.Enabled = false
		users.Fields.Add(
			&core.SelectField{Name: "group", Values: []string{"pgp1", "pgp2", "pgpx", "phd", "faculty", "staff", "exchange", "guest"}, MaxSelect: 1, Required: true},
			&core.SelectField{Name: "role", Values: []string{"member", "volunteer", "admin"}, MaxSelect: 1, Required: true},
			&core.TextField{Name: "college", Max: 120},
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
			&core.TextField{Name: "secret", Required: true, Hidden: true, Max: 64},
			&core.TextField{Name: "claimToken", Hidden: true, Max: 64},
			&core.SelectField{Name: "kind", Values: []string{"own", "guest", "exchange"}, MaxSelect: 1, Required: true},
			&core.SelectField{Name: "status", Values: []string{"SENT", "CLAIMED", "REVOKED"}, MaxSelect: 1, Required: true},
			&core.RelationField{Name: "holder", CollectionId: users.Id, MaxSelect: 1, CascadeDelete: true},
			&core.TextField{Name: "holderName", Required: true, Max: 120},
			&core.TextField{Name: "holderContact", Max: 120},
			&core.RelationField{Name: "issuer", CollectionId: users.Id, MaxSelect: 1, CascadeDelete: true},
			&core.NumberField{Name: "claimedAt", OnlyInt: true},
			&core.NumberField{Name: "revokedAt", OnlyInt: true},
			&core.NumberField{Name: "enteredAt", OnlyInt: true},
			&core.NumberField{Name: "enteredGate", OnlyInt: true},
			&core.RelationField{Name: "enteredBy", CollectionId: users.Id, MaxSelect: 1},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		passes.AddIndex("idx_passes_seq", true, "seq", "")
		passes.AddIndex("idx_passes_code", true, "code", "")
		passes.AddIndex("idx_passes_secret", true, "secret", "")
		passes.AddIndex("idx_passes_claim", true, "claimToken", "claimToken != ''")
		passes.AddIndex("idx_passes_own", true, "holder", "kind != 'guest' AND holder != ''")
		passes.AddIndex("idx_passes_issuer", false, "issuer", "")
		passes.AddIndex("idx_passes_holder", false, "holder", "")
		passes.AddIndex("idx_passes_contact", false, "holderContact", "")
		passes.AddIndex("idx_passes_entered", false, "enteredAt", "")
		if err := app.Save(passes); err != nil {
			return err
		}

		settings := core.NewBaseCollection("garba_settings")
		settings.Fields.Add(
			&core.JSONField{Name: "event", MaxSize: 10000},
			&core.JSONField{Name: "quotas", MaxSize: 10000},
		)
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
