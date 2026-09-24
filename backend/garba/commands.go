package garba

import (
	"fmt"
	"os"

	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

// Commands adds `exchange` and `seed` to the binary.
func Commands(app core.App, s *Service) []*cobra.Command {
	exchangeCmd := &cobra.Command{
		Use:   "exchange [guests.xlsx|guests.csv]",
		Short: "Import exchange guests (Name, Email, College, Phone) from an Excel or CSV file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			if err := app.RunAllMigrations(); err != nil {
				return err
			}
			res, err := s.ImportExchange(args[0], data)
			if err != nil {
				return err
			}
			fmt.Printf("%d added, %d updated, %d skipped\n", res.Created, res.Updated, len(res.Errors))
			for _, e := range res.Errors {
				fmt.Printf("  line %d: %s\n", e.Line, e.Reason)
			}
			return nil
		},
	}

	seedCmd := &cobra.Command{
		Use:   "seed",
		Short: "Add demo people and passes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RunAllMigrations(); err != nil {
				return err
			}
			people := []struct{ email, name, role string }{
				{"p25aarav@iima.ac.in", "Aarav Shah", "member"},
				{"p26diya@iima.ac.in", "Diya Rao", "member"},
				{"meena.k@iima.ac.in", "Meena Krishnan", "member"},
				{"p24meera@iima.ac.in", "Meera Iyer", "volunteer"},
				{"cultcomm@iima.ac.in", "Cultcomm Admin", "admin"},
			}
			for _, p := range people {
				u, err := s.SignInUser(p.email, p.name)
				if err != nil {
					return err
				}
				if role(u) != p.role {
					r := p.role
					if _, err := s.UpdatePerson(u.Id, PersonUpdate{Role: &r}); err != nil {
						return err
					}
				}
			}
			if _, err := s.ImportExchange("exchange.csv", []byte("Name,Email,College,Phone\nIshaan Mehta,ishaan.m@spjimr.org,SPJIMR Mumbai,+91 98200 00000\nTara Singh,,XLRI Jamshedpur,\n")); err != nil {
				return err
			}
			aarav, err := app.FindAuthRecordByEmail("users", "p25aarav@iima.ac.in")
			if err != nil {
				return err
			}
			if me, _ := s.Me(aarav); len(me.Guests) == 0 {
				for _, g := range [][2]string{{"Riya Patel", "riya.patel@gmail.com"}, {"Kabir Desai", ""}, {"Sunita Shah", ""}} {
					if _, err := s.AddGuest(aarav, g[0], g[1]); err != nil {
						return err
					}
				}
			}
			fmt.Println("Demo data ready: p25aarav (student, 3 guests), p26diya (PGP1), meena.k (faculty), p24meera (volunteer), cultcomm (admin) @iima.ac.in; exchange guest ishaan.m@spjimr.org.")
			return nil
		},
	}
	return []*cobra.Command{exchangeCmd, seedCmd}
}
