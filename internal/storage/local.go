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

	// 2. Move the file
	// We try os.Rename first (fast if on same partition)
	err := os.Rename(event.FilePath, destPath)
	if err != nil {
		// Fallback to copy and delete if Rename fails (e.g., across partitions)
		if err := s.copyFile(event.FilePath, destPath); err != nil {
			return "", fmt.Errorf("failed to copy file to local storage: %w", err)
		}
		if err := os.Remove(event.FilePath); err != nil {
			// Log but don't fail, as the file is safely in permanent storage
			// However, in this system, we want to know if cleanup fails.
			return destPath, fmt.Errorf("failed to remove source file after copy: %w", err)
		}
	}

	// 3. Return absolute path
	absPath, err := filepath.Abs(destPath)
	if err != nil {
		return destPath, nil // Return relative path if Abs fails
	}

	return absPath, nil
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
