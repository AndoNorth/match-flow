package matchstate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// pingRetries and pingBackoff bound how long Migrate waits for the
// database to accept connections before giving up. A freshly started
// Postgres container/pod can report its port as listening slightly
// before the network path actually forwards traffic to it (seen with
// both Docker and, potentially, K8s pod readiness); a bare connect
// then fails with "connection reset by peer" even though the database
// is up moments later. 5 attempts with linearly increasing backoff
// caps total wait around 2s, which comfortably covers that race.
const pingRetries = 5

var pingBackoff = 200 * time.Millisecond // ponytail: fixed step, tune if the race window grows

// Migrate applies every pending migration in migrations/ against dsn,
// via goose's Go library (never the CLI - the CLI, added to
// flake.nix in Task 1, is a local-dev-only tool for authoring new
// migrations and manual rollback). Safe to call every time the
// service starts: goose tracks what's already applied and is a no-op
// if nothing's pending.
func Migrate(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := pingWithRetry(ctx, db); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// pingWithRetry waits for db to accept connections, retrying a bounded
// number of times to absorb the container/pod-startup network race
// described above.
func pingWithRetry(ctx context.Context, db *sql.DB) error {
	var err error
	for i := 0; i < pingRetries; i++ {
		if err = db.PingContext(ctx); err == nil {
			return nil
		}
		if i < pingRetries-1 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context done while waiting to retry ping: %w", ctx.Err())
			case <-time.After(time.Duration(i+1) * pingBackoff):
			}
		}
	}
	return fmt.Errorf("ping failed after %d attempts: %w", pingRetries, err)
}
