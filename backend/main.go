// Garba Night passes for IIM Ahmedabad: a PocketBase app with custom routes.
//
//	go run . serve                    # dev, http://127.0.0.1:8090 (dashboard at /_/)
//	go build -o garba && ./garba serve --http 0.0.0.0:8090
//	./garba serve garba.iima.ac.in    # production with automatic HTTPS
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/osutils"

	"github.com/arnav127/iima-garba/backend/garba"
	_ "github.com/arnav127/iima-garba/backend/migrations"
)

func main() {
	app := pocketbase.New()

	var publicDir string
	app.RootCmd.PersistentFlags().StringVar(&publicDir, "publicDir", defaultPublicDir(), "the directory with the built web app")
	app.RootCmd.ParseFlags(os.Args[1:])

	s := garba.Register(app)
	app.RootCmd.AddCommand(garba.Commands(app, s)...)

	// Serve the built web app (npm run build writes it to pb_public) with SPA fallback.
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Func: func(e *core.ServeEvent) error {
			if !e.Router.HasRoute(http.MethodGet, "/{path...}") {
				static := apis.Static(os.DirFS(publicDir), true)
				e.Router.GET("/{path...}", func(re *core.RequestEvent) error {
					p := re.Request.URL.Path
					switch {
					case strings.HasPrefix(p, "/assets/"):
						re.Response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					case p == "/sw.js" || !strings.Contains(filepath.Base(p), "."):
						re.Response.Header().Set("Cache-Control", "no-cache")
					}
					return static(re)
				}).Bind(apis.Gzip())
			}
			return e.Next()
		},
		Priority: 999,
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func defaultPublicDir() string {
	if osutils.IsProbablyGoRun() {
		return "./pb_public"
	}
	return filepath.Join(os.Args[0], "../pb_public")
}
