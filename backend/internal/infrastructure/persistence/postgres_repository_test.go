package persistence

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"url-shortener/internal/domain"

	"github.com/jackc/pgx/v5/pgconn"
)

type recordedCall struct {
	query string
	args  []driver.Value
}

type stubDriverState struct {
	mu       sync.Mutex
	calls    []recordedCall
	execFn   func(query string, args []driver.NamedValue) (driver.Result, error)
	queryFn  func(query string, args []driver.NamedValue) (driver.Rows, error)
}

var (
	stubRegistryMu sync.Mutex
	stubStates     = make(map[string]*stubDriverState)
	stubSeq        int
)

func init() {
	sql.Register("microurl-stub-pg", &stubDriver{})
}

type stubDriver struct{}

func (d *stubDriver) Open(name string) (driver.Conn, error) {
	stubRegistryMu.Lock()
	state := stubStates[name]
	stubRegistryMu.Unlock()
	if state == nil {
		return nil, fmt.Errorf("missing stub state for %q", name)
	}
	return &stubConn{state: state}, nil
}

type stubConn struct {
	state *stubDriverState
}

func (c *stubConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (c *stubConn) Close() error                        { return nil }
func (c *stubConn) Begin() (driver.Tx, error)           { return nil, errors.New("not implemented") }

func (c *stubConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.state.record(query, args)
	if c.state.execFn != nil {
		return c.state.execFn(query, args)
	}
	return driver.RowsAffected(1), nil
}

func (c *stubConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.record(query, args)
	if c.state.queryFn != nil {
		return c.state.queryFn(query, args)
	}
	return &stubRows{}, nil
}

func (s *stubDriverState) record(query string, args []driver.NamedValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vals := make([]driver.Value, len(args))
	for i, a := range args {
		vals[i] = a.Value
	}
	s.calls = append(s.calls, recordedCall{query: query, args: vals})
}

type stubRows struct {
	cols []string
	data [][]driver.Value
	idx  int
}

func (r *stubRows) Columns() []string { return r.cols }
func (r *stubRows) Close() error      { return nil }
func (r *stubRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.idx])
	r.idx++
	return nil
}

func openStubDB(t *testing.T, state *stubDriverState) *sql.DB {
	t.Helper()
	stubRegistryMu.Lock()
	stubSeq++
	dsn := fmt.Sprintf("stub-%d", stubSeq)
	stubStates[dsn] = state
	stubRegistryMu.Unlock()

	t.Cleanup(func() {
		stubRegistryMu.Lock()
		delete(stubStates, dsn)
		stubRegistryMu.Unlock()
	})

	db, err := sql.Open("microurl-stub-pg", dsn)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestEnsureSchema(t *testing.T) {
	state := &stubDriverState{}
	db := openStubDB(t, state)

	if err := EnsureSchema(context.Background(), db); err != nil {
		t.Fatalf("EnsureSchema() unexpected error: %v", err)
	}
	if len(state.calls) != 5 {
		t.Fatalf("EnsureSchema() executed %d statements, want 5", len(state.calls))
	}
	if !strings.Contains(state.calls[0].query, "CREATE TABLE IF NOT EXISTS links") ||
		!strings.Contains(state.calls[3].query, "CREATE TABLE IF NOT EXISTS link_click_daily") {
		t.Fatalf("unexpected schema statements: %+v", state.calls)
	}

	failState := &stubDriverState{
		execFn: func(string, []driver.NamedValue) (driver.Result, error) {
			return nil, errors.New("permission denied")
		},
	}
	failDB := openStubDB(t, failState)
	if err := EnsureSchema(context.Background(), failDB); err == nil {
		t.Fatal("expected EnsureSchema() to propagate exec error")
	}
}

func TestPostgresRepository_Create(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	link := &domain.Link{
		ID:          "link-1",
		Slug:        "docs",
		OriginalURL: "https://example.com/docs",
		CreatedAt:   now,
	}

	state := &stubDriverState{}
	repo := NewPostgresRepository(openStubDB(t, state))
	if err := repo.Create(context.Background(), link); err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if len(state.calls) != 1 || !strings.Contains(state.calls[0].query, "INSERT INTO links") {
		t.Fatalf("unexpected Create query: %+v", state.calls)
	}

	conflictState := &stubDriverState{
		execFn: func(string, []driver.NamedValue) (driver.Result, error) {
			return nil, &pgconn.PgError{Code: "23505", Message: "duplicate key value"}
		},
	}
	conflictRepo := NewPostgresRepository(openStubDB(t, conflictState))
	if err := conflictRepo.Create(context.Background(), link); !errors.Is(err, domain.ErrInvalidSlug) {
		t.Fatalf("Create() duplicate err = %v, want errors.Is(_, domain.ErrInvalidSlug)", err)
	}
}

func TestPostgresRepository_FindBySlugAndExists(t *testing.T) {
	created := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	expires := created.Add(24 * time.Hour)

	state := &stubDriverState{
		queryFn: func(query string, _ []driver.NamedValue) (driver.Rows, error) {
			if strings.Contains(query, "SELECT EXISTS") {
				return &stubRows{
					cols: []string{"exists"},
					data: [][]driver.Value{{true}},
				}, nil
			}
			return &stubRows{
				cols: []string{"id", "slug", "original_url", "created_at", "expires_at", "click_count"},
				data: [][]driver.Value{{"id-1", "docs", "https://example.com", created, expires, int64(5)}},
			}, nil
		},
	}

	repo := NewPostgresRepository(openStubDB(t, state))
	got, err := repo.FindBySlug(context.Background(), "docs")
	if err != nil {
		t.Fatalf("FindBySlug() error: %v", err)
	}
	if got.ID != "id-1" || got.Slug != "docs" || got.ClickCount != 5 || got.ExpiresAt == nil || !got.ExpiresAt.Equal(expires) {
		t.Fatalf("FindBySlug() = %+v", got)
	}

	exists, err := repo.Exists(context.Background(), "docs")
	if err != nil || !exists {
		t.Fatalf("Exists() = (%v, %v), want (true, nil)", exists, err)
	}

	emptyState := &stubDriverState{
		queryFn: func(string, []driver.NamedValue) (driver.Rows, error) {
			return &stubRows{cols: []string{"id", "slug", "original_url", "created_at", "expires_at", "click_count"}}, nil
		},
	}
	emptyRepo := NewPostgresRepository(openStubDB(t, emptyState))
	if _, err := emptyRepo.FindBySlug(context.Background(), "missing"); !errors.Is(err, domain.ErrLinkNotFound) {
		t.Fatalf("FindBySlug(missing) err = %v, want domain.ErrLinkNotFound", err)
	}
}

func TestPostgresRepository_RecordClickAndGetStats(t *testing.T) {
	clickAt := time.Date(2026, 11, 16, 23, 30, 0, 0, time.FixedZone("UTC+2", 2*3600))
	created := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)

	state := &stubDriverState{
		queryFn: func(query string, _ []driver.NamedValue) (driver.Rows, error) {
			if strings.Contains(query, "FROM links WHERE slug") {
				return &stubRows{
					cols: []string{"id", "slug", "original_url", "created_at", "expires_at", "click_count"},
					data: [][]driver.Value{{"id-1", "launch", "https://example.com", created, nil, int64(9)}},
				}, nil
			}
			return &stubRows{
				cols: []string{"click_date", "clicks"},
				data: [][]driver.Value{
					{"2026-11-15", int64(4)},
					{"2026-11-16", int64(5)},
				},
			}, nil
		},
	}

	repo := NewPostgresRepository(openStubDB(t, state))
	if err := repo.RecordClick(context.Background(), "id-1", clickAt); err != nil {
		t.Fatalf("RecordClick() error: %v", err)
	}
	if len(state.calls) != 1 || state.calls[0].args[0] != "id-1" || state.calls[0].args[1] != "2026-11-16" {
		t.Fatalf("RecordClick() args = %#v, want linkID=id-1 UTC day=2026-11-16", state.calls)
	}

	stats, err := repo.GetStats(context.Background(), "launch", created, 7)
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if stats.TotalClicks != 9 || stats.Timezone != "UTC" || stats.RangeDays != 7 || stats.Status != "active" {
		t.Fatalf("GetStats() metadata = %+v", stats)
	}
	if len(stats.Daily) != 2 || stats.Daily[0].Date != "2026-11-15" || stats.Daily[1].Clicks != 5 {
		t.Fatalf("GetStats() daily = %+v", stats.Daily)
	}
}

// TestPostgresRepository_Integration exercises real SQL statements when TEST_DATABASE_URL is configured.
func TestPostgresRepository_Integration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run live PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open(pgx) error: %v", err)
	}
	defer db.Close()

	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatalf("EnsureSchema() error: %v", err)
	}

	repo := NewPostgresRepository(db)
	slug := fmt.Sprintf("it-%d", time.Now().UnixNano())
	link := &domain.Link{
		ID:          slug,
		Slug:        slug,
		OriginalURL: "https://example.com/integration",
		CreatedAt:   time.Now().UTC().Truncate(time.Second),
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM links WHERE id = $1`, link.ID)
	})

	if err := repo.Create(ctx, link); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if err := repo.Create(ctx, link); !errors.Is(err, domain.ErrInvalidSlug) {
		t.Fatalf("duplicate Create() err = %v, want domain.ErrInvalidSlug", err)
	}

	if err := repo.RecordClick(ctx, link.ID, time.Now().UTC()); err != nil {
		t.Fatalf("RecordClick() error: %v", err)
	}

	stats, err := repo.GetStats(ctx, link.Slug, time.Now().UTC().AddDate(0, 0, -6), 7)
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if stats.TotalClicks != 1 || len(stats.Daily) != 1 {
		t.Fatalf("unexpected integration stats: %+v", stats)
	}
}

