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

func (s *LocalStorage) Type() string {
	return "local"
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
	if err := s.copyFile(ctx, event.FilePath, destPath); err != nil {
		return "", fmt.Errorf("failed to copy file to local storage: %w", err)
	}

	// 3. Return a relative URL for the API
	// The pattern is: /artifacts/{dateDir}/{zone}/{eventID_filename}
	return fmt.Sprintf("/artifacts/%s/%s/%s_%s", dateDir, event.Zone, event.ID, fileName), nil
}

func (s *LocalStorage) copyFile(ctx context.Context, src, dst string) error {
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

	// Use a context-aware copy by periodically checking ctx.Done()
	// or using a pipe/goroutine for larger files. For simplicity and robustness
	// in this environment, we'll use a buffered copy loop.
	
	buffer := make([]byte, 32*1024) // 32KB buffer
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, err := sourceFile.Read(buffer)
			if n > 0 {
				if _, werr := destFile.Write(buffer[:n]); werr != nil {
					return werr
				}
			}
			if err != nil {
				if err == io.EOF {
					return destFile.Sync()
				}
				return err
			}
		}
	}
}
