// Garba Night passes for IIM Ahmedabad: a PocketBase app with custom routes. Sign-in comes from the
// students.iima.ac.in platform (iimakit/pb); this app decides who is on the list.
//
//	go run . serve                    # dev, http://127.0.0.1:8090 (dashboard at /_/)
//	go build -o garba && ./garba serve --http 127.0.0.1:8090
package main

import (
	"log"

	"github.com/pocketbase/pocketbase/core"

	iimakit "github.com/arnav127/iima-platform/kit"
	"github.com/arnav127/iima-platform/kit/pb"

	"github.com/arnav127/iima-garba/backend/garba"
	_ "github.com/arnav127/iima-garba/backend/migrations"
)

func main() {
	var s *garba.Service
	app := pb.New(pb.Config{
		AppID:   "garba2026",
		AppName: "Garba Night · IIMA",
		Access: func(e *core.RequestEvent, c *iimakit.Claims) (*core.Record, error) {
			return s.Access(garba.Person{
				Email: c.Email, Name: c.Name, Member: c.Member,
				Cohort: garba.Cohort(c.Affiliation, c.Programme, c.Year),
				Role:   garba.RoleFrom(c.Roles),
			})
		},
	})
	s = garba.Register(app)
	app.RootCmd.AddCommand(garba.Commands(app, s)...)

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
