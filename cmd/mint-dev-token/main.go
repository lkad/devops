// mint-dev-token: a small CLI that mints a dev JWT for the
// dev environment. Use it in smoke tests and the dev frontend
// to avoid going through the LDAP login flow.
//
//	go run ./cmd/mint-dev-token alice SuperAdmin
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/devops-toolkit/backend/internal/auth"
	"github.com/devops-toolkit/backend/pkg/contracts"
)

func main() {
	username := "alice"
	role := contracts.RoleSuperAdmin
	if len(os.Args) > 1 {
		username = os.Args[1]
	}
	if len(os.Args) > 2 {
		switch os.Args[2] {
		case "SuperAdmin":
			role = contracts.RoleSuperAdmin
		case "Operator":
			role = contracts.RoleOperator
		case "Developer":
			role = contracts.RoleDeveloper
		case "Auditor":
			role = contracts.RoleAuditor
		default:
			fmt.Fprintf(os.Stderr, "unknown role %q\n", os.Args[2])
			os.Exit(1)
		}
	}
	secret := os.Getenv("APP_JWT_SECRET")
	if secret == "" {
		secret = "dev-secret-do-not-use-in-prod"
	}
	s, err := auth.NewSigner(secret, time.Hour)
	if err != nil {
		fmt.Fprintf(os.Stderr, "signer: %v\n", err)
		os.Exit(1)
	}
	tok, _, err := s.Issue(&contracts.User{ID: username, Username: username, Role: role})
	if err != nil {
		fmt.Fprintf(os.Stderr, "issue: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(tok)
}
