# Test LDAP Setup

This repository contains a minimal test LDAP environment using the osixia/openldap Docker image. It prepopulates a small directory structure with example groups and users to help you develop and test your Go LDAP client.

## How it works

The `docker-compose.yml` defines a single LDAP service:

- **Image**: `osixia/openldap:1.5.0`
- **Environment**:
  - `LDAP_ORGANISATION`: Organization name
  - `LDAP_DOMAIN`: Base domain
  - `LDAP_ADMIN_PASSWORD`: Password for the `admin` DN
  - `LDAP_READONLY_USER`: Enables a read‑only user
  - `LDAP_READONLY_USER_USERNAME`: Username for the read‑only user
  - `LDAP_READONLY_USER_PASSWORD`: Password for the read‑only user
- **Ports**:
  - `389`: LDAP
  - `636`: LDAPS (disabled in this test config)
- **Volumes**:
  - `./bootstrap.ldif` is mounted into the container to preconfigure the directory
- **Restart**: `unless-stopped`

The `bootstrap.ldif` file creates the following structure:

```
dc=example,dc=com
  ├── ou=Groups
  │     ├── cn=IT_Ops
  │     │     └── member: cn=John,cn=Sarah
  │     ├── cn=DevTeam_Payments
  │     │     └── member: cn=Alice,cn=Bob
  │     ├── cn=Security_Auditors
  │     │     └── member: cn=Sarah
  │     └── cn=SRE_Lead
  │           └── member: cn=John
  └── ou=Users
        ├── cn=Alice
        ├── cn=Bob
        ├── cn=John
        └── cn=Sarah
```

Each user has a simple plaintext password (e.g., `alicepassword`). In a real environment you would store hashed passwords.

## How to use it

1. **Start the LDAP server**
   ```bash
   docker compose up -d
   ```
   This will create a container named `ldap-test` listening on `389`.

2. **Test connectivity**
   ```bash
   ldapsearch -x -H ldap://localhost:389 -D "cn=admin,dc=example,dc=com" -w adminpassword -b "dc=example,dc=com"
   ```

3. **Connect from Go**
   Use the `go-ldap` library to bind with `cn=admin,dc=example,dc=com` or any of the group DNs. The next step will be to implement the Go LDAP client.

4. **Stop the server**
   ```bash
   docker compose down
   ```

## Extending the setup

You can modify `bootstrap.ldif` to add more users, groups, or attributes. If you need to change the LDAP schema, adjust the Docker Compose environment variables accordingly.

## Security note

- **Do not expose this LDAP server to production traffic.** It is intended only for local testing.
- Change `LDAP_ADMIN_PASSWORD` to a strong secret before using in any environment that requires security.

---

For any questions or issues, feel free to open an issue or pull request.
