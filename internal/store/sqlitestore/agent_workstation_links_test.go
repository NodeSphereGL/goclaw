//go:build sqlite || sqliteonly

package sqlitestore

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func newLinkTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := OpenDB(filepath.Join(t.TempDir(), "links_test.db"))
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	if err := EnsureSchema(db); err != nil {
		db.Close()
		t.Fatalf("EnsureSchema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func seedLinkTenantAgent(t *testing.T, db *sql.DB) (tenantID, agentID uuid.UUID) {
	t.Helper()
	tenantID = uuid.Must(uuid.NewV7())
	agentID = uuid.Must(uuid.NewV7())
	if _, err := db.Exec(
		`INSERT INTO tenants (id, name, slug, status) VALUES (?,?,?,'active')`,
		tenantID.String(), "wl-"+tenantID.String()[:8], "wl"+tenantID.String()[:8],
	); err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO agents (id, tenant_id, agent_key, agent_type, status, provider, model, owner_id)
		 VALUES (?,?,?,'predefined','active','test','test-model','owner')`,
		agentID.String(), tenantID.String(), "wa-"+agentID.String()[:8],
	); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	return tenantID, agentID
}

func seedWorkstation(t *testing.T, db *sql.DB, tenantID uuid.UUID, key string) uuid.UUID {
	t.Helper()
	wsID := uuid.Must(uuid.NewV7())
	if _, err := db.Exec(
		`INSERT INTO workstations (id, workstation_key, tenant_id, name, backend_type, metadata, default_env)
		 VALUES (?,?,?,?, 'ssh', '{}', '{}')`,
		wsID.String(), key, tenantID.String(), key,
	); err != nil {
		t.Fatalf("seed workstation %s: %v", key, err)
	}
	return wsID
}

// seedLink inserts a link row directly with an explicit created_at so promotion
// ordering is deterministic in tests.
func seedLink(t *testing.T, db *sql.DB, agentID, wsID, tenantID uuid.UUID, isDefault bool, createdAt string) {
	t.Helper()
	def := 0
	if isDefault {
		def = 1
	}
	if _, err := db.Exec(
		`INSERT INTO agent_workstation_links (agent_id, workstation_id, tenant_id, is_default, created_at)
		 VALUES (?,?,?,?,?)`,
		agentID.String(), wsID.String(), tenantID.String(), def, createdAt,
	); err != nil {
		t.Fatalf("seed link: %v", err)
	}
}

// TestSetDefault_NotLinked_PreservesExistingDefault covers C1: SetDefault on a
// workstation the agent is not linked to must return ErrAgentWorkstationLinkNotFound
// and leave the prior default untouched (not clear it).
func TestSetDefault_NotLinked_PreservesExistingDefault(t *testing.T) {
	db := newLinkTestDB(t)
	tid, agent := seedLinkTenantAgent(t, db)
	ctx := sqliteTenantCtx(tid)
	s := NewSQLiteAgentWorkstationLinkStore(db)

	w1 := seedWorkstation(t, db, tid, "w1")
	w2 := seedWorkstation(t, db, tid, "w2")
	w3 := seedWorkstation(t, db, tid, "w3") // exists but NOT linked
	seedLink(t, db, agent, w1, tid, true, "2026-01-01T00:00:00.000Z")
	seedLink(t, db, agent, w2, tid, false, "2026-01-01T00:00:01.000Z")

	err := s.SetDefault(ctx, agent, w3)
	if !errors.Is(err, store.ErrAgentWorkstationLinkNotFound) {
		t.Fatalf("expected ErrAgentWorkstationLinkNotFound, got %v", err)
	}

	links, err := s.ListForAgent(ctx, agent)
	if err != nil {
		t.Fatalf("ListForAgent: %v", err)
	}
	assertSingleDefault(t, links, w1)
}

// TestSetDefault_Linked_SwitchesDefault verifies a valid SetDefault moves the
// default and clears the previous one (exactly one default remains).
func TestSetDefault_Linked_SwitchesDefault(t *testing.T) {
	db := newLinkTestDB(t)
	tid, agent := seedLinkTenantAgent(t, db)
	ctx := sqliteTenantCtx(tid)
	s := NewSQLiteAgentWorkstationLinkStore(db)

	w1 := seedWorkstation(t, db, tid, "w1")
	w2 := seedWorkstation(t, db, tid, "w2")
	seedLink(t, db, agent, w1, tid, true, "2026-01-01T00:00:00.000Z")
	seedLink(t, db, agent, w2, tid, false, "2026-01-01T00:00:01.000Z")

	if err := s.SetDefault(ctx, agent, w2); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	links, _ := s.ListForAgent(ctx, agent)
	assertSingleDefault(t, links, w2)
}

// TestUnlink_DefaultPromotesOldestRemaining covers C2: removing the default link
// on a multi-link agent promotes the oldest remaining link to default.
func TestUnlink_DefaultPromotesOldestRemaining(t *testing.T) {
	db := newLinkTestDB(t)
	tid, agent := seedLinkTenantAgent(t, db)
	ctx := sqliteTenantCtx(tid)
	s := NewSQLiteAgentWorkstationLinkStore(db)

	w1 := seedWorkstation(t, db, tid, "w1")
	w2 := seedWorkstation(t, db, tid, "w2")
	w3 := seedWorkstation(t, db, tid, "w3")
	seedLink(t, db, agent, w1, tid, true, "2026-01-01T00:00:00.000Z")  // default, oldest
	seedLink(t, db, agent, w2, tid, false, "2026-01-01T00:00:01.000Z") // oldest remaining
	seedLink(t, db, agent, w3, tid, false, "2026-01-01T00:00:02.000Z")

	if err := s.Unlink(ctx, agent, w1); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	links, _ := s.ListForAgent(ctx, agent)
	if len(links) != 2 {
		t.Fatalf("expected 2 remaining links, got %d", len(links))
	}
	assertSingleDefault(t, links, w2)
}

// TestUnlink_NonDefault_NoPromotion verifies removing a non-default link leaves
// the existing default intact and does not create a second default.
func TestUnlink_NonDefault_NoPromotion(t *testing.T) {
	db := newLinkTestDB(t)
	tid, agent := seedLinkTenantAgent(t, db)
	ctx := sqliteTenantCtx(tid)
	s := NewSQLiteAgentWorkstationLinkStore(db)

	w1 := seedWorkstation(t, db, tid, "w1")
	w2 := seedWorkstation(t, db, tid, "w2")
	seedLink(t, db, agent, w1, tid, true, "2026-01-01T00:00:00.000Z")
	seedLink(t, db, agent, w2, tid, false, "2026-01-01T00:00:01.000Z")

	if err := s.Unlink(ctx, agent, w2); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	links, _ := s.ListForAgent(ctx, agent)
	assertSingleDefault(t, links, w1)
}

// TestListForAgentWithWorkstation_JoinsFields covers C7: the joined query returns
// workstation display fields in a single call, ordered by created_at.
func TestListForAgentWithWorkstation_JoinsFields(t *testing.T) {
	db := newLinkTestDB(t)
	tid, agent := seedLinkTenantAgent(t, db)
	ctx := sqliteTenantCtx(tid)
	s := NewSQLiteAgentWorkstationLinkStore(db)

	w1 := seedWorkstation(t, db, tid, "router")
	w2 := seedWorkstation(t, db, tid, "builder")
	seedLink(t, db, agent, w1, tid, true, "2026-01-01T00:00:00.000Z")
	seedLink(t, db, agent, w2, tid, false, "2026-01-01T00:00:01.000Z")

	views, err := s.ListForAgentWithWorkstation(ctx, agent)
	if err != nil {
		t.Fatalf("ListForAgentWithWorkstation: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("expected 2 views, got %d", len(views))
	}
	if views[0].WorkstationID != w1 || views[0].WorkstationKey != "router" || !views[0].IsDefault {
		t.Fatalf("unexpected first view: %+v", views[0])
	}
	if views[0].BackendType != store.BackendSSH || !views[0].Active {
		t.Fatalf("view missing joined fields: %+v", views[0])
	}
	if views[1].WorkstationID != w2 || views[1].IsDefault {
		t.Fatalf("unexpected second view: %+v", views[1])
	}
}

func assertSingleDefault(t *testing.T, links []store.AgentWorkstationLink, wantDefault uuid.UUID) {
	t.Helper()
	var defaults []uuid.UUID
	for _, l := range links {
		if l.IsDefault {
			defaults = append(defaults, l.WorkstationID)
		}
	}
	if len(defaults) != 1 {
		t.Fatalf("expected exactly 1 default, got %d (%v)", len(defaults), defaults)
	}
	if defaults[0] != wantDefault {
		t.Fatalf("expected default %s, got %s", wantDefault, defaults[0])
	}
}
