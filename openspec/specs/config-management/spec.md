# config-management

## ADDED Requirements

### Requirement: YAML Configuration File
Application SHALL load configuration from `config.yaml`.

#### Scenario: Load config.yaml
- **WHEN** application starts
- **THEN** config.yaml is loaded from application directory
- **OR** from path specified in CONFIG_PATH env var

### Requirement: Environment Variable Override
Configuration values SHALL support environment variable overrides.

```yaml
database:
  password: "${DB_PASSWORD}"
```

#### Scenario: Env var override
- **WHEN** env var DB_PASSWORD is set
- **THEN** config.database.password uses that value
- **AND** placeholder ${DB_PASSWORD} is replaced

### Requirement: Nested Config Access
Configuration SHALL support nested struct access.

#### Scenario: Nested value access
- **WHEN** accessing config.database.host
- **THEN** value is retrieved from database.host in YAML

### Requirement: Required vs Optional Values
Config fields SHALL be marked as required or optional.

#### Scenario: Missing required value
- **WHEN** required config value is not set
- **THEN** application fails to start with clear error message

#### Scenario: Default value for optional
- **WHEN** optional config value is not set
- **THEN** default value from YAML is used

### Requirement: Config Validation
Configuration SHALL be validated at startup.

#### Scenario: Valid config
- **WHEN** all required values are present and valid
- **THEN** application starts successfully

#### Scenario: Invalid port number
- **WHEN** port value is not a valid port number
- **THEN** application fails with validation error

### Requirement: Sensitive Value Masking
Sensitive config values SHALL be masked in logs.

```yaml
database:
  password: "${DB_PASSWORD}"  # logged as "***"
```

#### Scenario: Password masking
- **WHEN** config is logged
- **THEN** password field shows "***"
- **AND** actual value is not exposed

### Requirement: Config Section Structure
Configuration SHALL have the following sections:

```yaml
app:           # Application settings
database:      # PostgreSQL connection
redis:         # Redis connection
logs:          # Log storage backend
ldap:          # LDAP authentication
alerts:        # Alert channels
k8s:           # Kubernetes settings
physicalhost: # Physical host monitoring
websocket:     # WebSocket settings
```

### Requirement: Config Hot Reload
Application SHALL support configuration hot reload without restart.

#### Scenario: Reload config
- **WHEN** SIGHUP is received
- **THEN** config is reloaded from file
- **AND** new values take effect for applicable components

### Requirement: Config Documentation
All config fields SHALL have documentation.

#### Scenario: Documented config
- **WHEN** config field is defined
- **THEN** Go struct tag provides description
- **AND** config.yaml.example includes comments

### Requirement: Default Config
Application SHALL provide sensible defaults.

#### Scenario: Default values
- **WHEN** config.yaml does not specify values
- **THEN** defaults are: port=8080, host=0.0.0.0, logs.backend=local
