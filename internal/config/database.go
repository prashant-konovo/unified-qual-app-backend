package config

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

// DBPair holds both database connections.
type DBPair struct {
	IRIS *sql.DB
	QS   *sql.DB
}

// ConnectDatabases opens connection pools for both IRIS and QS databases.
// Returns nil pools (not an error) when credentials are not configured,
// allowing the app to run in dummy mode.
func ConnectDatabases(cfg *Config) (*DBPair, error) {
	pair := &DBPair{}

	if cfg.IsDummy() {
		log.Println("[db] no credentials configured — running in dummy mode")
		return pair, nil
	}

	var err error

	if cfg.IRISDB.Password != "" {
		pair.IRIS, err = openDB("iris", cfg.IRISDB)
		if err != nil {
			return nil, fmt.Errorf("iris db: %w", err)
		}
	}

	if cfg.QSDB.Password != "" {
		pair.QS, err = openDB("qs", cfg.QSDB)
		if err != nil {
			return nil, fmt.Errorf("qs db: %w", err)
		}
	}

	return pair, nil
}

func openDB(label string, dbCfg DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", dbCfg.DSN())
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

	log.Printf("[db] %s connected (%s:%d/%s)", label, dbCfg.Host, dbCfg.Port, dbCfg.Name)
	return db, nil
}

// Close gracefully shuts down both pools.
func (d *DBPair) Close() {
	if d.IRIS != nil {
		d.IRIS.Close()
	}
	if d.QS != nil {
		d.QS.Close()
	}
}

// HealthCheck pings both databases and returns a per-db status map.
func (d *DBPair) HealthCheck(ctx context.Context) map[string]string {
	status := map[string]string{
		"incrowdDB": "not_configured",
		"qstoolDB":  "not_configured",
	}

	if d.IRIS != nil {
		if err := d.IRIS.PingContext(ctx); err != nil {
			status["incrowdDB"] = fmt.Sprintf("error: %v", err)
		} else {
			status["incrowdDB"] = "ok"
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
