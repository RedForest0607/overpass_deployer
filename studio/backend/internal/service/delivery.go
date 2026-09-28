package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"studio-backend/internal/localstore"
	"studio-backend/internal/repository"
	"time"
)

type Delivery struct {
	projects                          *Projects
	repo                              repository.DeliveryStore
	artifacts                         *localstore.Store
	packages                          *localstore.Store
	jarLimit, textLimit, previewLimit int64
	now                               func() time.Time
}

func NewDelivery(projects *Projects, repo repository.DeliveryStore, artifacts, packages *localstore.Store, jarLimit, textLimit, previewLimit int64) (*Delivery, error) {
	if projects == nil || repo == nil || artifacts == nil || packages == nil {
		return nil, errors.New("delivery dependencies are required")
	}
	return &Delivery{projects: projects, repo: repo, artifacts: artifacts, packages: packages, jarLimit: jarLimit, textLimit: textLimit, previewLimit: previewLimit, now: time.Now}, nil
}

func newID(prefix string) (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(value[:]), nil
}
