package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// DBPair holds all database connections.
type DBPair struct {
	IRIS         *sql.DB // InCrowd primary (read-write)
	IRISReadOnly *sql.DB // InCrowd read-replica
	QS           *sql.DB // QS-Tool
}

// ConnectDatabases opens connection pools for IRIS (primary + read-replica) and QS databases.
// Returns nil pools (not an error) when credentials are not configured,
// allowing the app to run in dummy mode.
// Connections are best-effort: the app starts even if databases are unreachable,
// and health checks report actual connectivity status.
func ConnectDatabases(cfg *config.Config) *DBPair {
	pair := &DBPair{}

	if cfg.IsDummy() {
		slog.Warn("no DB credentials configured, running in dummy mode")
		return pair
	}

	if cfg.IRISDB.Password != "" {
		db, err := openDB("iris", cfg.IRISDB)
		if err != nil {
			slog.Warn("iris primary connection failed, will retry on queries", "error", err)
		} else {
			pair.IRIS = db
		}

		roDB, err := openDB("iris-ro", cfg.IRISReadOnlyDB)
		if err != nil {
			slog.Warn("iris read-only failed, falling back to primary", "error", err)
			pair.IRISReadOnly = pair.IRIS
		} else {
			pair.IRISReadOnly = roDB
		}
	}

	if cfg.QSDB.Password != "" {
		db, err := openDB("qs", cfg.QSDB)
		if err != nil {
			slog.Warn("qs connection failed, will retry on queries", "error", err)
		} else {
			pair.QS = db
		}
	}

	return pair
}

func openDB(label string, dbCfg config.DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("mysql", dbCfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", label, err)
	}

	db.SetMaxOpenConns(dbCfg.MaxOpenConns)
	db.SetMaxIdleConns(dbCfg.MaxIdleConns)
	db.SetConnMaxLifetime(dbCfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping %s: %w", label, err)
	}

	slog.Info("database connected", "db", label, "host", dbCfg.Host, "port", dbCfg.Port, "schema", dbCfg.Name)
	return db, nil
}

// Close gracefully shuts down all pools.
func (d *DBPair) Close() {
	if d.IRIS != nil {
		d.IRIS.Close()
	}
	if d.IRISReadOnly != nil && d.IRISReadOnly != d.IRIS {
		d.IRISReadOnly.Close()
	}
	if d.QS != nil {
		d.QS.Close()
	}
}

// HealthCheck pings all databases and returns a per-db status map.
func (d *DBPair) HealthCheck(ctx context.Context) map[string]string {
	status := map[string]string{
		"incrowdDB":   "not_configured",
		"incrowdRODB": "not_configured",
		"qstoolDB":    "not_configured",
	}

	if d.IRIS != nil {
		if err := d.IRIS.PingContext(ctx); err != nil {
			status["incrowdDB"] = fmt.Sprintf("error: %v", err)
		} else {
			status["incrowdDB"] = "ok"
		}
	}

	if d.IRISReadOnly != nil {
		if err := d.IRISReadOnly.PingContext(ctx); err != nil {
			status["incrowdRODB"] = fmt.Sprintf("error: %v", err)
		} else {
			status["incrowdRODB"] = "ok"
		}
	}

	if d.QS != nil {
		if err := d.QS.PingContext(ctx); err != nil {
			status["qstoolDB"] = fmt.Sprintf("error: %v", err)
		} else {
			status["qstoolDB"] = "ok"
		}
	}

	return status
}
