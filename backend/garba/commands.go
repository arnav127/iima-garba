package garba

import (
	"fmt"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

// Commands adds `import` and `seed` to the binary.
func Commands(app core.App, s *Service) []*cobra.Command {
	importCmd := &cobra.Command{
		Use:   "import [people.csv]",
		Short: "Import people (email,name,group[,college][,role]) from a CSV file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			if err := app.RunAllMigrations(); err != nil {
				return err
			}
			res, err := s.ImportPeople(string(data))
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
		Short: "Add the demo people and passes from the design mockups",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RunAllMigrations(); err != nil {
				return err
			}
			s.Mail = func(to, subject, text string) {}
			if _, err := s.ImportPeople(`email,name,group,college,role
p25aarav@iima.ac.in,Aarav Shah,pgp1,,
p24meera@iima.ac.in,Meera Iyer,pgp2,,volunteer
cultcomm@iima.ac.in,Cultcomm Admin,staff,,admin
ishaan.m@spjimr.org,Ishaan Mehta,exchange,SPJIMR Mumbai,
`); err != nil {
				return err
			}
			aarav, err := app.FindAuthRecordByEmail("users", "p25aarav@iima.ac.in")
			if err != nil {
				return err
			}
			if me, _ := s.Me(aarav); len(me.Sent) == 0 {
				if _, err := s.SendPass(aarav, "Kabir Desai", "+91 98250 11223", "http://localhost:8090"); err != nil {
					return err
				}
				riya, err := s.SendPass(aarav, "Riya Patel", "riya.patel@gmail.com", "http://localhost:8090")
				if err != nil {
					return err
				}
				if _, err := s.Claim(riya.ClaimURL[strings.LastIndex(riya.ClaimURL, "/")+1:], "Riya Patel"); err != nil {
					return err
				}
			}
			fmt.Println("Demo data ready. Sign in with an email code (printed in the server log without SMTP):")
			fmt.Println("  p25aarav@iima.ac.in  student    ishaan.m@spjimr.org  exchange guest")
			fmt.Println("  p24meera@iima.ac.in  volunteer  cultcomm@iima.ac.in  admin")
			return nil
		},
	}
	return []*cobra.Command{importCmd, seedCmd}
}
