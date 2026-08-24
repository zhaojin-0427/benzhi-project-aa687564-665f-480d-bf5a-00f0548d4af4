package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const schemaVersion = 2

type Store struct {
	db           *sql.DB
	readOnly     bool
	loadCaseStmt *sql.Stmt
}

type IdempotencyRecord struct {
	Key         string
	CaseNumber  string
	Command     string
	Fingerprint string
	StatusCode  int
	Response    []byte
	CreatedAt   time.Time
}

func Open(path string) (*Store, error) {
	if path == "" {
		path = "stage-rigging-clearance.db"
	}
	dsn := path
	if path != ":memory:" {
		dsn = "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.initialize(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.Validate(context.Background()); err != nil {
		store.readOnly = true
		_ = db.Close()
		return nil, fmt.Errorf("存储完整性校验失败，已拒绝写入: %w", err)
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) initialize(ctx context.Context) error {
	statements := schemaStatements()
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("初始化数据库: %w", err)
		}
	}
	var version int
	err := s.db.QueryRowContext(ctx, "SELECT version FROM schema_meta WHERE id = 1").Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = s.db.ExecContext(ctx, "INSERT INTO schema_meta(id, version) VALUES(1, ?)", schemaVersion)
		return err
	}
	if err != nil {
		return err
	}
	if version == 1 {
		if _, err := s.db.ExecContext(ctx, "UPDATE schema_meta SET version=? WHERE id=1", schemaVersion); err != nil {
			return err
		}
		version = schemaVersion
	}
	if version != schemaVersion {
		return fmt.Errorf("不支持的 schemaVersion: %d", version)
	}
	return nil
}
