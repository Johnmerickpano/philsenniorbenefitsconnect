package database

import (
	"context"
	"fmt"

	"github.com/Johnmerickpano/philsenniorbenefitsconnect/internal/config"
	"github.com/gocql/gocql"
)

// NewScyllaSession creates a connection session to ScyllaDB.
func NewScyllaSession(cfg config.Config) (*gocql.Session, error) {
	if len(cfg.ScyllaHosts) == 0 {
		return nil, fmt.Errorf("no ScyllaDB hosts configured")
	}

	if cfg.ScyllaKeyspace == "" {
		return nil, fmt.Errorf("ScyllaDB keyspace is required")
	}

	if (cfg.ScyllaUsername == "") != (cfg.ScyllaPassword == "") {
		return nil, fmt.Errorf("both ScyllaDB username and password must be provided together")
	}

	cluster := gocql.NewCluster(cfg.ScyllaHosts...)
	cluster.Port = cfg.ScyllaPort
	cluster.Keyspace = cfg.ScyllaKeyspace
	cluster.Timeout = cfg.ScyllaConnectTimeout
	cluster.ConnectTimeout = cfg.ScyllaConnectTimeout
	cluster.NumConns = cfg.ScyllaNumConns

	if cfg.ScyllaUsername != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: cfg.ScyllaUsername,
			Password: cfg.ScyllaPassword,
		}
	}

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("connect to ScyllaDB: %w", err)
	}

	return session, nil
}

// Ping checks whether the ScyllaDB session can execute a query.
func Ping(ctx context.Context, session *gocql.Session) error {
	if session == nil {
		return fmt.Errorf("ScyllaDB session is nil")
	}

	var releaseVersion string
	if err := session.Query(
		"SELECT release_version FROM system.local",
	).WithContext(ctx).Scan(&releaseVersion); err != nil {
		return fmt.Errorf("ScyllaDB health check failed: %w", err)
	}

	return nil
}
