package garba

import "testing"

// Sign-in through the platform: membership, cohort and role come from the token.
func TestAccessFromThePlatform(t *testing.T) {
	s, app := setup(t)

	cases := []struct {
		name             string
		p                Person
		group, role      string
		ownPass, allowed bool
	}{
		{"PGP1", Person{Email: "p26diya@iima.ac.in", Member: true, Cohort: Cohort("student", "PGP", 1), Role: "member"}, "pgp1", "member", true, true},
		{"FABM2", Person{Email: "f25arjun@iima.ac.in", Member: true, Cohort: Cohort("student", "PGP-FABM", 2), Role: "member"}, "student", "member", true, true},
		{"PGPX is not PGP1", Person{Email: "x26rahul@iima.ac.in", Member: true, Cohort: Cohort("student", "PGPX", 1), Role: "member"}, "student", "member", true, true},
		{"staff", Person{Email: "meena.k@iima.ac.in", Member: true, Cohort: Cohort("faculty", "", 0), Role: "member"}, "faculty", "member", true, true},
		{"volunteer", Person{Email: "p25vol@iima.ac.in", Member: true, Cohort: Cohort("student", "PGP", 2), Role: RoleFrom([]string{"volunteer"})}, "student", "volunteer", true, true},
		{"admin without an IIMA address", Person{Email: "cultcomm.head@gmail.com", Role: RoleFrom([]string{"admin", "volunteer"})}, "guest", "admin", true, true},
		{"Gmail with no pass", Person{Email: "stranger@gmail.com", Role: "member"}, "", "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u, err := s.Access(c.p)
			if err != nil {
				t.Fatal(err)
			}
			if (u != nil) != c.allowed {
				t.Fatalf("allowed = %v", u != nil)
			}
			if u == nil {
				return
			}
			if group(u) != c.group || role(u) != c.role {
				t.Fatalf("group %q role %q", group(u), role(u))
			}
			if n, _ := app.CountRecords("passes"); c.ownPass && n == 0 {
				t.Fatal("no own pass")
			}
		})
	}

	// The platform is the source of truth for roles: losing "volunteer" there makes them a member
	// here on their next request.
	u, _ := s.Access(Person{Email: "p25vol@iima.ac.in", Member: true, Cohort: "student", Role: RoleFrom(nil)})
	if role(u) != "member" {
		t.Fatalf("role after removal: %q", role(u))
	}

	// An admin's move in the People tab (group locked) survives the platform's cohort.
	u.Set("group", "faculty")
	u.Set("groupLocked", true)
	if err := app.Save(u); err != nil {
		t.Fatal(err)
	}
	if u, _ = s.Access(Person{Email: "p25vol@iima.ac.in", Member: true, Cohort: "student", Role: "member"}); group(u) != "faculty" {
		t.Fatalf("locked group overwritten: %q", group(u))
	}
}
