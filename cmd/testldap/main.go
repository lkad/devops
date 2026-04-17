package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/example/devops/auth/ldap"
)

func main() {
	// Command line flags
	flag.Parse()

	if flag.NFlag() == 0 {
		printUsage()
		os.Exit(1)
	}

	// Get user credentials from environment or arguments
	userDN := os.Getenv("TEST_LDAP_USER_DN")
	password := os.Getenv("TEST_LDAP_PASSWORD")

	if userDN == "" || password == "" {
		fmt.Println("Error: TEST_LDAP_USER_DN and TEST_LDAP_PASSWORD environment variables are required")
		printUsage()
		os.Exit(1)
	}

	// Create LDAP client
	config := ldap.DefaultConfig()
	config.URL = getEnvOrDefault("LDAP_URL", "ldap://localhost:389")
	config.BaseDN = getEnvOrDefault("LDAP_BASE_DN", "dc=example,dc=com")
	config.AdminDN = getEnvOrDefault("LDAP_ADMIN_DN", "cn=admin,"+config.BaseDN)
	config.AdminPassword = getEnvOrDefault("LDAP_ADMIN_PASSWORD", "admin")

	client, err := ldap.NewClient(config)
	if err != nil {
		fmt.Printf("Error creating LDAP client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Authenticate
	fmt.Printf("Authenticating user: %s\n", userDN)
	success, err := client.Authenticate(userDN, password)
	if err != nil {
		fmt.Printf("Authentication error: %v\n", err)
		os.Exit(1)
	}

	if success {
		fmt.Println("✓ Authentication successful")
	} else {
		fmt.Println("✗ Authentication failed")
		os.Exit(1)
	}

	// Get groups
	fmt.Printf("\nRetrieving groups for: %s\n", userDN)
	groups, err := client.GetGroups(userDN)
	if err != nil {
		fmt.Printf("Error getting groups: %v\n", err)
		os.Exit(1)
	}

	if len(groups) == 0 {
		fmt.Println("  No groups found")
	} else {
		for _, group := range groups {
			fmt.Printf("  - %s\n", group)
		}
	}

	// Get roles
	fmt.Printf("\nResolving roles for: %s\n", userDN)
	roles, err := client.GetRoles(userDN)
	if err != nil {
		fmt.Printf("Error getting roles: %v\n", err)
		os.Exit(1)
	}

	if len(roles) == 0 {
		fmt.Println("  No roles found")
	} else {
		for _, role := range roles {
			fmt.Printf("  - %s\n", role)
		}
	}

	fmt.Println("\n✓ Done")
}

func printUsage() {
	fmt.Println("LDAP Authentication Demo CLI")
	fmt.Println("")
	fmt.Println("Usage: go run cmd/testldap/main.go [options]")
	fmt.Println("")
	fmt.Println("Environment Variables:")
	fmt.Println("  TEST_LDAP_USER_DN     User DN to authenticate (e.g., cn=john,ou=Users,dc=example,dc=com)")
	fmt.Println("  TEST_LDAP_PASSWORD    User password")
	fmt.Println("  LDAP_URL              LDAP server URL (default: ldap://localhost:389)")
	fmt.Println("  LDAP_BASE_DN          Base DN (default: dc=example,dc=com)")
	fmt.Println("  LDAP_ADMIN_DN         Admin DN (default: cn=admin,dc=example,dc=com)")
	fmt.Println("  LDAP_ADMIN_PASSWORD   Admin password (default: admin)")
	fmt.Println("")
	fmt.Println("Example:")
	fmt.Println("  export TEST_LDAP_USER_DN=\"cn=john,ou=Users,dc=example,dc=com\"")
	fmt.Println("  export TEST_LDAP_PASSWORD=\"password123\"")
	fmt.Println("  go run cmd/testldap/main.go")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
