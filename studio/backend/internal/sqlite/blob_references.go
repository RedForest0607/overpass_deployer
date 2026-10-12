package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type referenceRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}

// ReferencedBlobKeys is a fail-closed snapshot: callers must not receive a
// partial reference set when iteration, decoding, or closing fails.
func (r *DeliveryRepository) ReferencedBlobKeys(ctx context.Context) (map[string]struct{}, map[string]struct{}, error) {
	artifacts, packages := map[string]struct{}{}, map[string]struct{}{}
	queries := []struct {
		sql    string
		keys   map[string]struct{}
		decode func(string) (string, error)
	}{
		{"SELECT blob_key FROM artifacts", artifacts, directBlobKey},
		{"SELECT manifest_json FROM packages", packages, packageZIPBlobKey},
		{"SELECT blob_key FROM package_files WHERE blob_key<>''", packages, directBlobKey},
	}
	for _, query := range queries {
		rows, err := r.db.QueryContext(ctx, query.sql)
		if err != nil {
			return nil, nil, err
		}
		if err := collectBlobReferences(rows, query.keys, query.decode); err != nil {
			return nil, nil, fmt.Errorf("load blob references: %w", err)
		}
	}
	return artifacts, packages, nil
}

func collectBlobReferences(rows referenceRows, keys map[string]struct{}, decode func(string) (string, error)) (resultErr error) {
	defer func() {
		if err := rows.Close(); resultErr == nil {
			resultErr = err
		}
	}()
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return err
		}
		key, err := decode(value)
		if err != nil {
			return err
		}
		keys[key] = struct{}{}
	}
	return rows.Err()
}

func directBlobKey(value string) (string, error) {
	if value == "" {
		return "", errors.New("empty blob reference; cleanup must not proceed")
	}
	return value, nil
}

func packageZIPBlobKey(manifest string) (string, error) {
	var metadata map[string]string
	if err := json.Unmarshal([]byte(manifest), &metadata); err != nil {
		return "", fmt.Errorf("decode package manifest: %w", err)
	}
	return directBlobKey(metadata["zipBlobKey"])
}
