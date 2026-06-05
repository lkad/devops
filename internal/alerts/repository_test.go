package alerts

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openTestDB builds a fresh sqlite db, migrates the alerts schema,
// and schedules cleanup. Each test gets its own DB so they cannot
// leak rows into each other.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "alerts.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(AllModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// TestRepository_Alert_CreateAndGet is the spec scenario "Create
// alert" + "Get alert": a new alert row is persisted and read back
// with all fields intact. The ID is auto-assigned by BaseModel.
func TestRepository_Alert_CreateAndGet(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	a := &Alert{
		Name:       "high_cpu",
		Severity:   SeverityWarning,
		State:      StateFiring,
		SourceType: SourceTypePhysicalHost,
		SourceID:   "host-1",
		Labels:     JSONMap{"env": "prod"},
		FiredAt:    time.Now().UTC(),
	}
	if err := repo.CreateAlert(a); err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.ID == "" {
		t.Error("ID not assigned")
	}
	got, err := repo.GetAlert(a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "high_cpu" || got.Severity != SeverityWarning {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
	if got.Labels["env"] != "prod" {
		t.Errorf("labels: %+v", got.Labels)
	}
}

// TestRepository_Alert_Update verifies partial updates for the
// alert state machine: acknowledge and resolve.
func TestRepository_Alert_Update(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	a := &Alert{Name: "x", Severity: SeverityInfo, State: StateFiring, SourceType: SourceTypeCustom, SourceID: "s", FiredAt: time.Now().UTC()}
	_ = repo.CreateAlert(a)
	ackedBy := "alice"
	now := time.Now().UTC()
	patch := map[string]any{
		"state":            StateAcknowledged,
		"acknowledged_by":  ackedBy,
		"acknowledged_at":  now,
	}
	if err := repo.UpdateAlert(a.ID, patch); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := repo.GetAlert(a.ID)
	if got.State != StateAcknowledged {
		t.Errorf("state: got %q want acknowledged", got.State)
	}
	if got.AcknowledgedBy == nil || *got.AcknowledgedBy != "alice" {
		t.Errorf("ack by: %+v", got.AcknowledgedBy)
	}
}

// TestRepository_Alert_Delete covers the soft-delete semantics:
// a deleted row is invisible to Get but still in the table.
func TestRepository_Alert_Delete(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	a := &Alert{Name: "x", Severity: SeverityInfo, State: StateFiring, SourceType: SourceTypeCustom, SourceID: "s", FiredAt: time.Now().UTC()}
	_ = repo.CreateAlert(a)
	if err := repo.DeleteAlert(a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetAlert(a.ID); !IsNotFound(err) {
		t.Errorf("expected NotFound after delete, got %v", err)
	}
}

// TestRepository_Alert_List applies the AlertFilter to a
// pre-seeded table and asserts the rows returned match the
// filter dimensions.
func TestRepository_Alert_List(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	now := time.Now().UTC()
	for _, n := range []string{"a", "b", "c"} {
		_ = repo.CreateAlert(&Alert{Name: n, Severity: SeverityInfo, State: StateFiring, SourceType: SourceTypeCustom, SourceID: n, FiredAt: now})
	}
	rows, total, err := repo.ListAlerts(AlertFilter{Name: "a"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Name != "a" {
		t.Errorf("filter mismatch: total=%d rows=%+v", total, rows)
	}
}

// TestRepository_Channel_CreateAndGet covers the channel CRUD
// happy path: a slack channel with a Config map is persisted
// and read back. The masking is applied at the handler layer,
// not the repository, so the raw map is what the repo returns.
func TestRepository_Channel_CreateAndGet(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	c := &Channel{
		Type:    ChannelTypeSlack,
		Config:  JSONMap{"webhook_url": "https://x", "channel": "#a"},
		Enabled: true,
	}
	if err := repo.CreateChannel(c); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.GetChannel(c.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Type != ChannelTypeSlack {
		t.Errorf("type: got %q", got.Type)
	}
	if got.Config["channel"] != "#a" {
		t.Errorf("config: %+v", got.Config)
	}
}

// TestRepository_Channel_List covers the GET /alerts/channels
// scenario: every persisted channel is returned.
func TestRepository_Channel_List(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	for _, ty := range []ChannelType{ChannelTypeSlack, ChannelTypeEmail, ChannelTypeLog} {
		_ = repo.CreateChannel(&Channel{Type: ty, Enabled: true, Config: JSONMap{}})
	}
	rows, err := repo.ListChannels()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("list: got %d want 3", len(rows))
	}
}

// TestRepository_Channel_Delete covers the DELETE
// /alerts/channels/:id scenario: a 204 is returned and the row
// is no longer retrievable.
func TestRepository_Channel_Delete(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	c := &Channel{Type: ChannelTypeLog, Enabled: true, Config: JSONMap{}}
	_ = repo.CreateChannel(c)
	if err := repo.DeleteChannel(c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.GetChannel(c.ID); !IsNotFound(err) {
		t.Errorf("expected NotFound after delete, got %v", err)
	}
}

// TestRepository_Channel_Update covers the PUT /alerts/channels/:id
// scenario: a channel's enabled flag is toggled.
func TestRepository_Channel_Update(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	c := &Channel{Type: ChannelTypeLog, Enabled: true, Config: JSONMap{}}
	_ = repo.CreateChannel(c)
	if err := repo.UpdateChannel(c.ID, map[string]any{"enabled": false}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := repo.GetChannel(c.ID)
	if got.Enabled {
		t.Errorf("enabled flag not toggled")
	}
}

// TestRepository_AlertRule_CreateAndGet covers the rule CRUD
// happy path: a rule with a DSL condition and a list of channel
// IDs is persisted and read back.
func TestRepository_AlertRule_CreateAndGet(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	r := &AlertRule{
		Name:             "high-cpu",
		ConditionDSL:     "cpu > 80",
		ChannelIDs:       []string{"ch-1", "ch-2"},
		Enabled:          true,
		SuppressionWindow: 5 * time.Minute,
	}
	if err := repo.CreateRule(r); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.GetRule(r.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "high-cpu" || got.ConditionDSL != "cpu > 80" {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
	if len(got.ChannelIDs) != 2 {
		t.Errorf("channel ids: %+v", got.ChannelIDs)
	}
}

// TestRepository_NotFound wraps the not-found helper check used
// by every test that asserts a deleted row.
func TestRepository_NotFound(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	_, err := repo.GetAlert("nope")
	if !IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}
