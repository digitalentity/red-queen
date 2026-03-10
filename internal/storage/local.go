package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"redqueen/internal/config"
	"redqueen/internal/models"
)

type LocalStorage struct {
	cfg config.LocalConfig
}

func NewLocalStorage(cfg config.LocalConfig) *LocalStorage {
	return &LocalStorage{
		cfg: cfg,
	}
}

func (s *LocalStorage) Save(ctx context.Context, event *models.Event) (string, error) {
	// 1. Prepare destination path
	// Structure: root_path/YYYY-MM-DD/zone/eventID_filename
	dateDir := event.Timestamp.Format("2006-01-02")
	destDir := filepath.Join(s.cfg.RootPath, dateDir, event.Zone)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	fileName := filepath.Base(event.FilePath)
	destPath := filepath.Join(destDir, fmt.Sprintf("%s_%s", event.ID, fileName))

	// 2. Copy the file (Always copy, never move, to keep storage provider generic)
	if err := s.copyFile(event.FilePath, destPath); err != nil {
		return "", fmt.Errorf("failed to copy file to local storage: %w", err)
	}

	// 3. Return a relative URL for the API
	// The pattern is: /artifacts/{dateDir}/{zone}/{eventID_filename}
	return fmt.Sprintf("/artifacts/%s/%s/%s_%s", dateDir, event.Zone, event.ID, fileName), nil
}

func (s *LocalStorage) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}
