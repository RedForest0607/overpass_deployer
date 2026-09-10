package service

import (
	"context"
	"encoding/json"
	"studio-backend/internal/domain"
)

func (s *Projects) Validate(ctx context.Context, projectID string, raw json.RawMessage, requestID string) (domain.Validation, error) {
	if _, err := s.Get(ctx, projectID); err != nil {
		return domain.Validation{}, err
	}
	layout := json.RawMessage(`{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}`)
	if err := domain.ValidateTopologyStructure(raw, layout, s.maxNodes, s.maxConnections); err != nil {
		return domain.Validation{}, &InvalidInput{Err: err}
	}
	return domain.ValidateDesign(raw, requestID), nil
}
