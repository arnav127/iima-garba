package garba

import (
	"log"
	"net/http"
	"net/mail"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mailer"
)

// Register wires the Garba routes and auth hooks into the PocketBase app.
func Register(app core.App) *Service {
	s := New(app)
	s.AppURL = strings.TrimRight(os.Getenv("APP_URL"), "/")

	// Only people on the list (roster, exchange list, or friends who were sent a pass) can sign in.
	app.OnRecordAuthWithOAuth2Request("users").BindFunc(func(e *core.RecordAuthWithOAuth2RequestEvent) error {
		if e.IsNewRecord || e.Record == nil {
			return apiErr(http.StatusForbidden, "This Google account isn't on the Garba Night list. IIMA students and staff: sign in with the email on the Cultcomm list. Guests: use the email your pass was sent to.")
		}
		if e.Record.GetString("name") == "" && e.OAuth2User != nil {
			e.Record.Set("name", e.OAuth2User.Name)
		}
		return e.Next()
	})
	app.OnRecordRequestOTPRequest("users").BindFunc(func(e *core.RecordCreateOTPRequestEvent) error {
		if e.Record == nil {
			return apiErr(http.StatusNotFound, "This email isn't on the Garba Night list. IIMA students and staff use the email on the Cultcomm list; guests use the email their pass was sent to.")
		}
		return e.Next()
	})
	// Without SMTP (local dev), print sign-in codes to the console instead of sending them.
	app.OnMailerRecordOTPSend("users").BindFunc(func(e *core.MailerRecordEvent) error {
		if !e.App.Settings().SMTP.Enabled {
			log.Printf("[dev] sign-in code for %s: %v (SMTP not configured, so it was not emailed)", e.Record.Email(), e.Meta["password"])
			return nil
		}
		return e.Next()
	})

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		if err := applyEnvConfig(app); err != nil {
			return err
		}
		r := se.Router
		auth := apis.RequireAuth("users")
		appURL := func(e *core.RequestEvent) string {
			if s.AppURL != "" {
				return s.AppURL
			}
			if o := e.Request.Header.Get("Origin"); o != "" {
				return o
			}
			scheme := "http"
			if e.IsTLS() || e.Request.Header.Get("X-Forwarded-Proto") == "https" {
				scheme = "https"
			}
			return scheme + "://" + e.Request.Host
		}
		type body struct {
			Name    string `json:"name"`
			Contact string `json:"contact"`
			Payload string `json:"payload"`
			Gate    int    `json:"gate"`
			Role    string `json:"role"`
			CSV     string `json:"csv"`
		}
		bind := func(e *core.RequestEvent) (body, error) {
			var b body
			if err := e.BindBody(&b); err != nil {
				return b, apiErr(400, "Invalid request")
			}
			return b, nil
		}
		ok := func(e *core.RequestEvent) error { return e.JSON(200, map[string]bool{"ok": true}) }

		g := r.Group("/api/garba")

		// public
		g.GET("/event", func(e *core.RequestEvent) error {
			e.Response.Header().Set("Cache-Control", "public, max-age=30")
			return e.JSON(200, s.EventInfo())
		})
		g.GET("/claim/{token}", func(e *core.RequestEvent) error {
			out, err := s.ClaimInfo(e.Request.PathValue("token"))
			if err != nil {
				return err
			}
			return e.JSON(200, out)
		})
		g.POST("/claim/{token}", func(e *core.RequestEvent) error {
			b, err := bind(e)
			if err != nil {
				return err
			}
			out, err := s.Claim(e.Request.PathValue("token"), b.Name)
			if err != nil {
				return err
			}
			return e.JSON(200, out)
		})

		// signed in
		g.GET("/me", func(e *core.RequestEvent) error {
			out, err := s.Me(e.Auth)
			if err != nil {
				return err
			}
			return e.JSON(200, out)
		}).Bind(auth)
		g.POST("/passes", func(e *core.RequestEvent) error {
			b, err := bind(e)
			if err != nil {
				return err
			}
			out, err := s.SendPass(e.Auth, b.Name, b.Contact, appURL(e))
			if err != nil {
				return err
			}
			return e.JSON(200, out)
		}).Bind(auth)
		g.GET("/passes/{id}/link", func(e *core.RequestEvent) error {
			link, err := s.ClaimLink(e.Auth, e.Request.PathValue("id"), appURL(e))
			if err != nil {
				return err
			}
			return e.JSON(200, map[string]string{"claimUrl": link})
		}).Bind(auth)
		g.POST("/passes/{id}/revoke", func(e *core.RequestEvent) error {
			if err := s.RevokeSent(e.Auth, e.Request.PathValue("id")); err != nil {
				return err
			}
			out, err := s.Me(e.Auth)
			if err != nil {
				return err
			}
			return e.JSON(200, out)
		}).Bind(auth)

		// gate
		gate := g.Group("/scan").Bind(auth).BindFunc(RequireRole("volunteer", "admin"))
		gate.POST("", func(e *core.RequestEvent) error {
			b, err := bind(e)
			if err != nil {
				return err
			}
			out, err := s.Scan(e.Auth, b.Payload, b.Gate)
			if err != nil {
				return err
			}
			return e.JSON(200, out)
		})
		gate.GET("/count", func(e *core.RequestEvent) error {
			return e.JSON(200, map[string]int{"entered": s.EnteredCount(), "gates": s.Settings().Event.Gates})
		})

		// admin
		adm := g.Group("/admin").Bind(auth).BindFunc(RequireRole("admin"))
		adm.GET("/stats", func(e *core.RequestEvent) error {
			out, err := s.Stats()
			if err != nil {
				return err
			}
			return e.JSON(200, out)
		})
		adm.GET("/passes", func(e *core.RequestEvent) error {
			q := e.Request.URL.Query()
			out, err := s.AdminPasses(q.Get("q"), q.Get("filter"))
			if err != nil {
				return err
			}
			return e.JSON(200, out)
		})
		adm.GET("/people", func(e *core.RequestEvent) error {
			out, err := s.AdminPeople(e.Request.URL.Query().Get("q"))
			if err != nil {
				return err
			}
			return e.JSON(200, out)
		})
		adm.POST("/import", func(e *core.RequestEvent) error {
			b, err := bind(e)
			if err != nil {
				return err
			}
			out, err := s.ImportPeople(b.CSV)
			if err != nil {
				return err
			}
			return e.JSON(200, out)
		})
		adm.POST("/passes/{id}/revoke", func(e *core.RequestEvent) error {
			if err := s.AdminRevoke(e.Request.PathValue("id")); err != nil {
				return err
			}
			return ok(e)
		})
		adm.POST("/passes/{id}/restore", func(e *core.RequestEvent) error {
			if err := s.AdminRestore(e.Request.PathValue("id")); err != nil {
				return err
			}
			return ok(e)
		})
		adm.POST("/passes/{id}/undo-entry", func(e *core.RequestEvent) error {
			if err := s.AdminUndoEntry(e.Request.PathValue("id")); err != nil {
				return err
			}
			return ok(e)
		})
		adm.POST("/people/{id}/role", func(e *core.RequestEvent) error {
			b, err := bind(e)
			if err != nil {
				return err
			}
			if e.Request.PathValue("id") == e.Auth.Id {
				return apiErr(400, "You can't change your own role")
			}
			if err := s.AdminSetRole(e.Request.PathValue("id"), b.Role); err != nil {
				return err
			}
			return ok(e)
		})
		adm.GET("/settings", func(e *core.RequestEvent) error { return e.JSON(200, s.Settings()) })
		adm.PUT("/settings", func(e *core.RequestEvent) error {
			var in Settings
			if err := e.BindBody(&in); err != nil {
				return apiErr(400, "Invalid settings")
			}
			out, err := s.SaveSettings(in)
			if err != nil {
				return err
			}
			return e.JSON(200, out)
		})
		adm.GET("/export.csv", func(e *core.RequestEvent) error {
			data, err := s.ExportCSV()
			if err != nil {
				return err
			}
			e.Response.Header().Set("Content-Disposition", `attachment; filename="garba-passes.csv"`)
			return e.Blob(200, "text/csv; charset=utf-8", data)
		})

		return se.Next()
	})
	return s
}

func (s *Service) sendMail(to, subject, text string) {
	st := s.app.Settings()
	if !st.SMTP.Enabled {
		log.Printf("[dev] email to %s (SMTP not configured, not sent)\nSubject: %s\n%s", to, subject, text)
		return
	}
	msg := &mailer.Message{
		From:    mail.Address{Name: st.Meta.SenderName, Address: st.Meta.SenderAddress},
		To:      []mail.Address{{Address: to}},
		Subject: subject,
		Text:    text,
	}
	if err := s.app.NewMailClient().Send(msg); err != nil {
		s.app.Logger().Error("Failed to send email", "to", to, "error", err)
	}
}
