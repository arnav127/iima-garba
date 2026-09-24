package garba

import (
	"os"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// applyEnvConfig copies deployment settings from environment variables into PocketBase
// on every start, so a campus server can be configured with an env file alone.
// Anything left unset keeps whatever was configured in the PocketBase dashboard (/_/).
//
//	APP_URL                 public URL of the web app (claim links, email links)
//	GOOGLE_CLIENT_ID        Google OAuth client for "Sign in with Google"
//	GOOGLE_CLIENT_SECRET
//	SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, SMTP_TLS=1
//	MAIL_FROM_ADDRESS, MAIL_FROM_NAME
//	ADMIN_EMAILS            comma separated; these people are always Cultcomm admins
func applyEnvConfig(app core.App) error {
	env := os.Getenv

	settings := app.Settings()
	changed := false
	if v := env("APP_URL"); v != "" && settings.Meta.AppURL != v {
		settings.Meta.AppURL, changed = v, true
	}
	if settings.Meta.AppName != "Garba Night · IIMA" {
		settings.Meta.AppName, changed = "Garba Night · IIMA", true
	}
	if v := env("MAIL_FROM_ADDRESS"); v != "" {
		settings.Meta.SenderAddress, changed = v, true
	}
	if v := env("MAIL_FROM_NAME"); v != "" {
		settings.Meta.SenderName, changed = v, true
	} else if settings.Meta.SenderName == "Support" || settings.Meta.SenderName == "" {
		settings.Meta.SenderName, changed = "Cultcomm IIMA", true
	}
	if host := env("SMTP_HOST"); host != "" {
		port, _ := strconv.Atoi(env("SMTP_PORT"))
		if port == 0 {
			port = 587
		}
		settings.SMTP.Enabled = true
		settings.SMTP.Host = host
		settings.SMTP.Port = port
		settings.SMTP.Username = env("SMTP_USERNAME")
		settings.SMTP.Password = env("SMTP_PASSWORD")
		settings.SMTP.TLS = env("SMTP_TLS") == "1"
		changed = true
	}
	if changed {
		if err := app.Save(settings); err != nil {
			return err
		}
	}

	if id, secret := env("GOOGLE_CLIENT_ID"), env("GOOGLE_CLIENT_SECRET"); id != "" && secret != "" {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		cfg := core.OAuth2ProviderConfig{Name: "google", ClientId: id, ClientSecret: secret}
		found := false
		for i, p := range users.OAuth2.Providers {
			if p.Name == "google" {
				users.OAuth2.Providers[i], found = cfg, true
			}
		}
		if !found {
			users.OAuth2.Providers = append(users.OAuth2.Providers, cfg)
		}
		users.OAuth2.Enabled = true
		if err := app.Save(users); err != nil {
			return err
		}
	}

	for _, email := range strings.Split(env("ADMIN_EMAILS"), ",") {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}
		err := app.RunInTransaction(func(tx core.App) error {
			u, err := tx.FindAuthRecordByEmail("users", email)
			if err != nil {
				if u, err = newUser(tx, email, strings.Split(email, "@")[0], "staff", "admin", ""); err != nil {
					return err
				}
				return ensureOwnPass(tx, u)
			}
			if role(u) != "admin" {
				u.Set("role", "admin")
				return tx.Save(u)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
