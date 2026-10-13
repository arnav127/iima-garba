package garba

import (
	"os"
	"regexp"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// applyEnvConfig copies deployment settings from environment variables into PocketBase
// on every start, so a campus server can be configured with an env file alone.
// Anything left unset keeps whatever was configured in the PocketBase dashboard (/_/).
//
//	APP_URL                  public URL of the web app (used in pass links)
//	MEMBER_DOMAINS           email domains that get a pass on sign-in (default iima.ac.in)
//	GOOGLE_CLIENT_ID         "Sign in with Google"
//	GOOGLE_CLIENT_SECRET
//	MICROSOFT_CLIENT_ID      optional "Sign in with Microsoft", for exchange colleges whose mail is on Microsoft 365
//	MICROSOFT_CLIENT_SECRET
//	MICROSOFT_TENANT         optional: one organisation's domain to allow only that one (default: any organisation)
//	ADMIN_EMAILS             comma separated; these people are always Cultcomm admins
//	TRUSTED_PROXY_HEADERS    e.g. X-Forwarded-For behind Apache/nginx, so logs and rate limits see real client IPs
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
	if v := env("TRUSTED_PROXY_HEADERS"); v != "" && strings.Join(settings.TrustedProxy.Headers, ",") != v {
		settings.TrustedProxy.Headers = strings.Split(v, ",")
		settings.TrustedProxy.UseLeftmostIP = false // the proxy appends the real client IP on the right
		changed = true
	}
	if changed {
		if err := app.Save(settings); err != nil {
			return err
		}
	}

	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	usersChanged := false
	for _, p := range []struct{ name, id, secret string }{
		{"google", env("GOOGLE_CLIENT_ID"), env("GOOGLE_CLIENT_SECRET")},
		{"microsoft", env("MICROSOFT_CLIENT_ID"), env("MICROSOFT_CLIENT_SECRET")},
	} {
		if p.id == "" || p.secret == "" {
			continue
		}
		cfg := core.OAuth2ProviderConfig{Name: p.name, ClientId: p.id, ClientSecret: p.secret}
		if p.name == "microsoft" {
			// Work and school accounts of any organisation (each exchange college has its own), but not
			// personal Microsoft accounts: those can't be tied to a college. See microsoftEmail.
			base := "https://login.microsoftonline.com/" + microsoftTenant() + "/oauth2/v2.0/"
			cfg.AuthURL, cfg.TokenURL = base+"authorize", base+"token"
		}
		found := false
		for i, existing := range users.OAuth2.Providers {
			if existing.Name == p.name {
				users.OAuth2.Providers[i], found = cfg, true
			}
		}
		if !found {
			users.OAuth2.Providers = append(users.OAuth2.Providers, cfg)
		}
		usersChanged = true
	}
	if usersChanged {
		users.OAuth2.Enabled = true
		if err := app.Save(users); err != nil {
			return err
		}
	}
	return nil
}

var tenantRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)

// microsoftTenant is which Microsoft organisations may sign in: "organizations" (any college or
// company, the default) or one organisation's domain or directory ID (MICROSOFT_TENANT) to allow
// just that one. "common" and "consumers" would let personal accounts in, so they are not accepted.
func microsoftTenant() string {
	t := strings.ToLower(strings.TrimSpace(os.Getenv("MICROSOFT_TENANT")))
	if !tenantRe.MatchString(t) || t == "common" || t == "consumers" {
		return "organizations"
	}
	return t
}

// ensureAdmins makes sure the ADMIN_EMAILS people exist and are admins.
func (s *Service) ensureAdmins() error {
	for _, email := range strings.Split(os.Getenv("ADMIN_EMAILS"), ",") {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}
		u, err := s.app.FindAuthRecordByEmail("users", email)
		if err != nil {
			if !s.isMemberEmail(email) {
				// Non-IIMA admins get an account (and, below, their own pass).
				if u, err = newUser(s.app, email, strings.Split(email, "@")[0], "guest", "admin"); err != nil {
					return err
				}
			} else if u, err = s.SignInUser(email, ""); err != nil {
				return err
			}
		}
		if role(u) != "admin" {
			u.Set("role", "admin")
			if err := s.app.Save(u); err != nil {
				return err
			}
		}
		if err := ensureOwnPass(s.app, u); err != nil {
			return err
		}
	}
	return nil
}
