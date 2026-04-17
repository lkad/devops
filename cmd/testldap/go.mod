module github.com/example/devops/cmd/testldap

go 1.21

require github.com/example/devops/auth/ldap v0.0.0

replace github.com/example/devops/auth/ldap => ../auth/ldap
