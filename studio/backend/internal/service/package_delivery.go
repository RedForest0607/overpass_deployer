package service

import (
	"context"
	"errors"
	"io"
	"studio-backend/internal/domain"
)

func (s *Delivery) GetPackage(ctx context.Context, projectID, id string) (domain.Package, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return domain.Package{}, err
	}
	return s.repo.GetPackage(ctx, ownerID, projectID, id)
}
func (s *Delivery) OpenPackage(ctx context.Context, projectID, id string) (io.ReadCloser, domain.Package, error) {
	pkg, err := s.GetPackage(ctx, projectID, id)
	if err != nil {
		return nil, domain.Package{}, err
	}
	reader, err := s.packages.Open(pkg.ZIPBlobKey)
	return reader, pkg, err
}
func (s *Delivery) PreviewFile(ctx context.Context, projectID, packageID, fileID string) (string, bool, error) {
	ownerID, err := requestOwner(ctx)
	if err != nil {
		return "", false, err
	}
	file, err := s.repo.GetPackageFile(ctx, ownerID, projectID, packageID, fileID)
	if err != nil {
		return "", false, err
	}
	if !file.Previewable {
		return "", false, &InvalidInput{Err: errors.New("file is not previewable")}
	}
	reader, err := s.packages.Open(file.BlobKey)
	if err != nil {
		return "", false, err
	}
	defer reader.Close()
	body, err := io.ReadAll(io.LimitReader(reader, s.previewLimit+1))
	if err != nil {
		return "", false, err
	}
	truncated := int64(len(body)) > s.previewLimit
	if truncated {
		body = body[:s.previewLimit]
	}
	return string(body), truncated, nil
}
