package garba

import (
	"io"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// Register wires the Garba routes and sign-in hooks into the PocketBase app.
func Register(app core.App) *Service {
	s := New(app)
	s.AppURL = strings.TrimRight(os.Getenv("APP_URL"), "/")
	if d := os.Getenv("MEMBER_DOMAINS"); d != "" {
		s.MemberDomains = nil
		for _, v := range strings.Split(d, ",") {
			if v = strings.ToLower(strings.TrimSpace(v)); v != "" {
				s.MemberDomains = append(s.MemberDomains, v)
			}
		}
	}

	// Sign-in is OAuth2 only (Google, optionally Microsoft). We decide who gets in and create
	// their account ourselves, then PocketBase links the provider to it.
	app.OnRecordAuthWithOAuth2Request("users").BindFunc(func(e *core.RecordAuthWithOAuth2RequestEvent) error {
		if e.OAuth2User == nil || e.OAuth2User.Email == "" {
			return ErrNotOnList
		}
		u, err := s.SignInUser(e.OAuth2User.Email, e.OAuth2User.Name)
		if err != nil {
			return err
		}
		e.Record, e.IsNewRecord = u, false
		return e.Next()
	})

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		if err := applyEnvConfig(app); err != nil {
			return err
		}
		if err := s.ensureAdmins(); err != nil {
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
			Email   string `json:"email"`
			Payload string `json:"payload"`
			PassID  string `json:"passId"`
			Gate    int    `json:"gate"`
			Role    string `json:"role"`
		}
		bind := func(e *core.RequestEvent) (body, error) {
			var b body
			if err := e.BindBody(&b); err != nil {
				return b, apiErr(400, "Invalid request")
			}
			return b, nil
		}
		reply := func(e *core.RequestEvent, v any, err error) error {
			if err != nil {
				return err
			}
			return e.JSON(200, v)
		}
		ok := func(e *core.RequestEvent, err error) error { return reply(e, map[string]bool{"ok": true}, err) }
		csvFile := func(e *core.RequestEvent, name string, data []byte, err error) error {
			if err != nil {
				return err
			}
			e.Response.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
			return e.Blob(200, "text/csv; charset=utf-8", data)
		}

		g := r.Group("/api/garba")

		// public
		g.GET("/event", func(e *core.RequestEvent) error {
			e.Response.Header().Set("Cache-Control", "public, max-age=30")
			return e.JSON(200, s.Settings().Event)
		})
		g.GET("/p/{token}", func(e *core.RequestEvent) error {
			out, err := s.PassByLink(e.Request.PathValue("token"))
			return reply(e, out, err)
		})

		// signed in
		g.GET("/me", func(e *core.RequestEvent) error {
			out, err := s.Me(e.Auth)
			return reply(e, out, err)
		}).Bind(auth)
		g.POST("/guests", func(e *core.RequestEvent) error {
			b, err := bind(e)
			if err != nil {
				return err
			}
			out, err := s.AddGuest(e.Auth, b.Name, b.Email)
			return reply(e, out, err)
		}).Bind(auth)
		g.POST("/guests/{id}/remove", func(e *core.RequestEvent) error {
			out, err := s.RemoveGuest(e.Auth, e.Request.PathValue("id"))
			return reply(e, out, err)
		}).Bind(auth)
		g.GET("/guests/{id}/link", func(e *core.RequestEvent) error {
			link, err := s.GuestLink(e.Auth, e.Request.PathValue("id"), appURL(e))
			return reply(e, map[string]string{"link": link}, err)
		}).Bind(auth)

		// gate
		gate := g.Group("/scan").Bind(auth).BindFunc(RequireRole("volunteer", "admin"))
		gate.POST("", func(e *core.RequestEvent) error {
			b, err := bind(e)
			if err != nil {
				return err
			}
			out, err := s.Scan(e.Auth, b.Payload, b.Gate)
			return reply(e, out, err)
		})
		gate.POST("/admit", func(e *core.RequestEvent) error {
			b, err := bind(e)
			if err != nil {
				return err
			}
			out, err := s.Admit(e.Auth, b.PassID, b.Gate)
			return reply(e, out, err)
		})
		gate.GET("/count", func(e *core.RequestEvent) error {
			return e.JSON(200, map[string]int{"entered": s.EnteredCount(), "gates": s.Settings().Event.Gates})
		})

		// admin
		adm := g.Group("/admin").Bind(auth).BindFunc(RequireRole("admin"))
		adm.GET("/stats", func(e *core.RequestEvent) error {
			out, err := s.Stats()
			return reply(e, out, err)
		})
		adm.GET("/passes", func(e *core.RequestEvent) error {
			q := e.Request.URL.Query()
			out, err := s.AdminPasses(q.Get("q"), q.Get("filter"))
			return reply(e, out, err)
		})
		adm.GET("/people", func(e *core.RequestEvent) error {
			out, err := s.AdminPeople(e.Request.URL.Query().Get("q"))
			return reply(e, out, err)
		})
		adm.POST("/people/{id}", func(e *core.RequestEvent) error {
			var in PersonUpdate
			if err := e.BindBody(&in); err != nil {
				return apiErr(400, "Invalid request")
			}
			if e.Request.PathValue("id") == e.Auth.Id && in.Role != nil && *in.Role != "admin" {
				return apiErr(400, "You can't remove your own admin access")
			}
			out, err := s.UpdatePerson(e.Request.PathValue("id"), in)
			return reply(e, out, err)
		})
		adm.POST("/access", func(e *core.RequestEvent) error {
			b, err := bind(e)
			if err != nil {
				return err
			}
			out, err := s.GrantAccess(b.Email, b.Role)
			return reply(e, out, err)
		})
		adm.POST("/exchange", func(e *core.RequestEvent) error {
			files, err := e.FindUploadedFiles("file")
			if err != nil || len(files) == 0 {
				return apiErr(400, "Choose an .xlsx or .csv file")
			}
			f, err := files[0].Reader.Open()
			if err != nil {
				return err
			}
			defer f.Close()
			data, err := io.ReadAll(io.LimitReader(f, 10<<20))
			if err != nil {
				return err
			}
			out, err := s.ImportExchange(files[0].OriginalName, data)
			return reply(e, out, err)
		}).Bind(apis.BodyLimit(10 << 20))
		adm.GET("/exchange-links.csv", func(e *core.RequestEvent) error {
			data, err := s.ExchangeLinks(appURL(e))
			return csvFile(e, "exchange-pass-links.csv", data, err)
		})
		adm.POST("/passes/{id}/revoke", func(e *core.RequestEvent) error { return ok(e, s.AdminRevoke(e.Request.PathValue("id"))) })
		adm.POST("/passes/{id}/restore", func(e *core.RequestEvent) error { return ok(e, s.AdminRestore(e.Request.PathValue("id"))) })
		adm.POST("/passes/{id}/undo-entry", func(e *core.RequestEvent) error { return ok(e, s.AdminUndoEntry(e.Request.PathValue("id"))) })
		adm.GET("/settings", func(e *core.RequestEvent) error { return e.JSON(200, s.Settings()) })
		adm.PUT("/settings", func(e *core.RequestEvent) error {
			var in SettingsInput
			if err := e.BindBody(&in); err != nil {
				return apiErr(400, "Invalid settings")
			}
			out, err := s.SaveSettings(in)
			return reply(e, out, err)
		})
		adm.GET("/export.csv", func(e *core.RequestEvent) error {
			data, err := s.ExportCSV()
			return csvFile(e, "garba-passes.csv", data, err)
		})

		return se.Next()
	})
	return s
}
