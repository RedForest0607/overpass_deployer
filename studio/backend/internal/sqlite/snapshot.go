package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
)

type serializer interface {
	Serialize() ([]byte, error)
}

// Snapshot returns a transactionally consistent image of an existing SQLite
// database. SQLite includes committed WAL pages in the serialized image.
func Snapshot(ctx context.Context, databasePath string) ([]byte, error) {
	info, err := os.Lstat(databasePath)
	if err != nil {
		return nil, fmt.Errorf("inspect sqlite database: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("sqlite database must be a regular file")
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite snapshot source: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	connection, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire sqlite snapshot connection: %w", err)
	}
	defer connection.Close()
	var snapshot []byte
	err = connection.Raw(func(driverConnection any) error {
		value, ok := driverConnection.(serializer)
		if !ok {
			return errors.New("sqlite driver does not support serialization")
		}
		var serializeErr error
		snapshot, serializeErr = value.Serialize()
		return serializeErr
	})
	if err != nil {
		return nil, fmt.Errorf("serialize sqlite database: %w", err)
	}
	return snapshot, nil
}

func CheckIntegrity(ctx context.Context, databasePath string) error {
	dsn := (&url.URL{Scheme: "file", Path: databasePath, RawQuery: "mode=ro&immutable=1"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open restored sqlite database: %w", err)
	}
	defer db.Close()
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("check restored sqlite database: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("restored sqlite integrity check failed: %s", result)
	}
	return nil
}
