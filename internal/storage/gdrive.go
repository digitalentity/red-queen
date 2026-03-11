package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"redqueen/internal/config"
	"redqueen/internal/models"
)

// gdriveResumableThreshold is the file size above which resumable uploads are used.
const gdriveResumableThreshold = 5 * 1024 * 1024 // 5 MiB

// fileCreator is a narrow interface over the Drive Files API to allow test injection.
type fileCreator interface {
	create(ctx context.Context, file *drive.File, content io.Reader, size int64) (webViewLink string, err error)
}

// GDriveStorage is a storage.Provider that uploads artifacts to Google Drive.
// Uploaded files are private and inherit the sharing settings of the parent folder.
// The folder must be shared with the service account as Editor.
type GDriveStorage struct {
	creator  fileCreator
	folderID string
}

// NewGDriveStorage authenticates using the service account key at cfg.CredentialsFile
// and returns a GDriveStorage ready to upload into cfg.FolderID.
func NewGDriveStorage(ctx context.Context, cfg config.GDriveConfig) (*GDriveStorage, error) {
	data, err := os.ReadFile(cfg.CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("read gdrive credentials: %w", err)
	}
	creds, err := google.CredentialsFromJSON(ctx, data, drive.DriveFileScope)
	if err != nil {
		return nil, fmt.Errorf("parse gdrive credentials: %w", err)
	}
	svc, err := drive.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("create drive service: %w", err)
	}
	return &GDriveStorage{
		creator:  &driveFileCreator{svc: svc},
		folderID: cfg.FolderID,
	}, nil
}

// newGDriveStorageWithCreator constructs a GDriveStorage with an injected fileCreator.
// For use in tests only.
func newGDriveStorageWithCreator(creator fileCreator, folderID string) *GDriveStorage {
	return &GDriveStorage{creator: creator, folderID: folderID}
}

// Type implements Provider.
func (g *GDriveStorage) Type() string { return "google_drive" }

// Save uploads the artifact file to Google Drive and returns the webViewLink.
// Files smaller than gdriveResumableThreshold use a simple multipart upload;
// larger files use a resumable upload.
func (g *GDriveStorage) Save(ctx context.Context, event *models.Event) (string, error) {
	info, err := os.Stat(event.FilePath)
	if err != nil {
		return "", fmt.Errorf("stat file for gdrive upload: %w", err)
	}

	f, err := os.Open(event.FilePath)
	if err != nil {
		return "", fmt.Errorf("open file for gdrive upload: %w", err)
	}
	defer f.Close()

	name := fmt.Sprintf("%s_%s_%s",
		event.Timestamp.Format("20060102T150405"),
		event.Zone,
		filepath.Base(event.FilePath),
	)
	meta := &drive.File{
		Name:    name,
		Parents: []string{g.folderID},
	}

	link, err := g.creator.create(ctx, meta, f, info.Size())
	if err != nil {
		return "", fmt.Errorf("gdrive upload: %w", err)
	}
	return link, nil
}

// driveFileCreator is the production fileCreator backed by a real drive.Service.
type driveFileCreator struct {
	svc *drive.Service
}

func (c *driveFileCreator) create(ctx context.Context, file *drive.File, content io.Reader, size int64) (string, error) {
	call := c.svc.Files.Create(file).Fields("id,webViewLink").Context(ctx)
	if size >= gdriveResumableThreshold {
		ra, ok := content.(io.ReaderAt)
		if !ok {
			return "", fmt.Errorf("content does not implement io.ReaderAt, required for resumable upload")
		}
		call = call.ResumableMedia(ctx, ra, size, "application/octet-stream")
	} else {
		call = call.Media(content)
	}
	created, err := call.Do()
	if err != nil {
		return "", err
	}
	return created.WebViewLink, nil
}
