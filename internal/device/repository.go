package device

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(dsn string) (*Repository, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	repo := &Repository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *Repository) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		labels JSONB DEFAULT '{}',
		business_unit TEXT,
		compute_cluster TEXT,
		parent_id TEXT,
		config JSONB DEFAULT '{}',
		metadata JSONB DEFAULT '{}',
		registered_at TIMESTAMP,
		last_seen TIMESTAMP,
		last_config_sync TIMESTAMP,
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status);
	CREATE INDEX IF NOT EXISTS idx_devices_type ON devices(type);
	CREATE INDEX IF NOT EXISTS idx_devices_labels ON devices USING GIN(labels);
	`

	_, err := r.db.Exec(schema)
	return err
}

func (r *Repository) Create(d *Device) error {
	query := `
		INSERT INTO devices (id, type, name, status, labels, business_unit, compute_cluster, parent_id, config, metadata, registered_at, last_seen, last_config_sync)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	labelsJSON, _ := json.Marshal(d.Labels)
	configJSON, _ := json.Marshal(d.Config)
	metadataJSON, _ := json.Marshal(d.Metadata)
	_, err := r.db.Exec(query, d.ID, d.Type, d.Name, d.Status, labelsJSON, d.BusinessUnit, d.ComputeCluster, d.ParentID, configJSON, metadataJSON, d.RegisteredAt, d.LastSeen, d.LastConfigSync)
	return err
}

func (r *Repository) GetByID(id string) (*Device, error) {
	query := `
		SELECT id, type, name, status, labels, business_unit, compute_cluster, parent_id, config, metadata, registered_at, last_seen, last_config_sync, created_at, updated_at
		FROM devices WHERE id = $1
	`
	d := &Device{}
	var labels, config, metadata []byte
	err := r.db.QueryRow(query, id).Scan(
		&d.ID, &d.Type, &d.Name, &d.Status, &labels, &d.BusinessUnit, &d.ComputeCluster, &d.ParentID, &config, &metadata, &d.RegisteredAt, &d.LastSeen, &d.LastConfigSync, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal(labels, &d.Labels)
	json.Unmarshal(config, &d.Config)
	json.Unmarshal(metadata, &d.Metadata)
	return d, nil
}

func (r *Repository) List() ([]*Device, error) {
	query := `
		SELECT id, type, name, status, labels, business_unit, compute_cluster, parent_id, config, metadata, registered_at, last_seen, last_config_sync, created_at, updated_at
		FROM devices ORDER BY created_at DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		d := &Device{}
		var labels, config, metadata []byte
		if err := rows.Scan(&d.ID, &d.Type, &d.Name, &d.Status, &labels, &d.BusinessUnit, &d.ComputeCluster, &d.ParentID, &config, &metadata, &d.RegisteredAt, &d.LastSeen, &d.LastConfigSync, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(labels, &d.Labels)
			json.Unmarshal(config, &d.Config)
			json.Unmarshal(metadata, &d.Metadata)
			devices = append(devices, d)
	}
	return devices, nil
}

func (r *Repository) ListPaginated(limit, offset int) ([]*Device, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM devices`
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get paginated results
	query := `
		SELECT id, type, name, status, labels, business_unit, compute_cluster, parent_id, config, metadata, registered_at, last_seen, last_config_sync, created_at, updated_at
		FROM devices ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		d := &Device{}
		var labels, config, metadata []byte
		if err := rows.Scan(&d.ID, &d.Type, &d.Name, &d.Status, &labels, &d.BusinessUnit, &d.ComputeCluster, &d.ParentID, &config, &metadata, &d.RegisteredAt, &d.LastSeen, &d.LastConfigSync, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, 0, err
		}
		json.Unmarshal(labels, &d.Labels)
		json.Unmarshal(config, &d.Config)
		json.Unmarshal(metadata, &d.Metadata)
		devices = append(devices, d)
	}
	return devices, total, nil
}

func (r *Repository) Update(d *Device) error {
	query := `
		UPDATE devices SET
			type = $2, name = $3, status = $4, labels = $5, business_unit = $6, compute_cluster = $7, parent_id = $8, config = $9, metadata = $10, last_seen = $11, last_config_sync = $12, updated_at = $13
		WHERE id = $1
	`
	d.UpdatedAt = time.Now()
	labelsJSON, _ := json.Marshal(d.Labels)
	configJSON, _ := json.Marshal(d.Config)
	metadataJSON, _ := json.Marshal(d.Metadata)
	_, err := r.db.Exec(query, d.ID, d.Type, d.Name, d.Status, labelsJSON, d.BusinessUnit, d.ComputeCluster, d.ParentID, configJSON, metadataJSON, d.LastSeen, d.LastConfigSync, d.UpdatedAt)
	return err
}

func (r *Repository) Delete(id string) error {
	query := `DELETE FROM devices WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *Repository) UpdateStatus(id string, status State) error {
	query := `UPDATE devices SET status = $2, updated_at = $3 WHERE id = $1`
	_, err := r.db.Exec(query, id, status, time.Now())
	return err
}

func (r *Repository) SearchByLabels(labels map[string]string) ([]*Device, error) {
	query := `SELECT id, type, name, status, labels, business_unit, compute_cluster, parent_id, config, metadata, registered_at, last_seen, last_config_sync, created_at, updated_at FROM devices WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	for k, v := range labels {
		query += fmt.Sprintf(" AND labels->>'%s' = $%d", k, argIdx)
		args = append(args, v)
		argIdx++
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		d := &Device{}
		var labels, config, metadata []byte
		if err := rows.Scan(&d.ID, &d.Type, &d.Name, &d.Status, &labels, &d.BusinessUnit, &d.ComputeCluster, &d.ParentID, &config, &metadata, &d.RegisteredAt, &d.LastSeen, &d.LastConfigSync, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(labels, &d.Labels)
			json.Unmarshal(config, &d.Config)
			json.Unmarshal(metadata, &d.Metadata)
			devices = append(devices, d)
	}
	return devices, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}
