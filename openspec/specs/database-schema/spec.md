# database-schema

## ADDED Requirements

### Requirement: Versioned Migrations
Database changes SHALL be managed through versioned migration files.

#### Scenario: Migration file naming
- **WHEN** new migration is created
- **THEN** file is named `NNNNNN_{description}.up.sql`
- **AND** corresponding `NNNNNN_{description}.down.sql` exists
- **AND** NNNNNN is zero-padded 6-digit sequence number

#### Scenario: Migration execution
- **WHEN** `migrate up` is run
- **THEN** migrations are applied in sequence order

#### Scenario: Migration rollback
- **WHEN** `migrate down` is run
- **THEN** last migration is rolled back

### Requirement: Primary Keys
All tables SHALL use UUID as primary key.

```sql
id UUID PRIMARY KEY DEFAULT uuid_generate_v4()
```

#### Scenario: UUID primary key
- **WHEN** new table is created
- **THEN** id column is UUID type
- **AND** default is uuid_generate_v4()

### Requirement: Timestamps
All tables SHALL include created_at and updated_at timestamps.

```sql
created_at TIMESTAMPTZ DEFAULT NOW()
updated_at TIMESTAMPTZ DEFAULT NOW()
```

#### Scenario: Auto-updated timestamp
- **WHEN** row is updated via UPDATE command
- **THEN** updated_at is automatically set to current time

### Requirement: Soft Deletes
Tables that may need data recovery SHALL use soft deletes.

```sql
deleted_at TIMESTAMPTZ  -- NULL means not deleted
```

#### Scenario: Soft delete query
- **WHEN** fetching active records
- **THEN** query includes `WHERE deleted_at IS NULL`

### Requirement: JSONB for Metadata
Semi-structured data SHALL use JSONB columns.

```sql
metadata JSONB DEFAULT '{}'
labels JSONB DEFAULT '{}'
```

#### Scenario: JSONB indexing
- **WHEN** JSONB column needs querying
- **THEN** GIN index is created for the column

### Requirement: Foreign Key Constraints
Relationships SHALL be enforced with foreign key constraints.

```sql
parent_id UUID REFERENCES devices(id)
```

#### Scenario: Cascade delete
- **WHEN** parent row is deleted
- **THEN** child rows are cascade deleted if ON DELETE CASCADE specified

### Requirement: Unique Constraints
Fields that must be unique SHALL have explicit UNIQUE constraints.

```sql
username VARCHAR(255) UNIQUE NOT NULL
```

### Requirement: Check Constraints
Valid values SHALL be enforced with CHECK constraints.

```sql
state VARCHAR(50) NOT NULL CHECK (state IN ('pending', 'active', 'retire'))
```

### Requirement: Indexes
Frequently queried columns SHALL have indexes.

```sql
CREATE INDEX idx_devices_state ON devices(state);
CREATE INDEX idx_devices_labels ON devices USING GIN(labels);
```

### Requirement: Migration Testing
Migrations SHALL be tested before deployment.

#### Scenario: Dry run
- **WHEN** migration is applied with dry-run
- **THEN** SQL is validated without execution

### Requirement: Zero Downtime Migrations
Migrations SHALL be designed for zero downtime deployment.

#### Scenario: Adding column
- **WHEN** adding new NOT NULL column
- **THEN** column is added with default value first
- **AND** default is backfilled in separate migration

#### Scenario: Dropping column
- **WHEN** dropping column
- **THEN** column is dropped in separate migration after code deployment
