//go:build integration

package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"redqueen/internal/models"
	"redqueen/internal/storage/fakedriveserver"
)

func newFakeDriveStorage(t *testing.T, fake *fakedriveserver.Server, folderID string) *GDriveStorage {
	t.Helper()
	svc, err := drive.NewService(context.Background(),
		option.WithHTTPClient(fake.Client()),
		option.WithoutAuthentication(),
		option.WithEndpoint(fake.URL()+"/"),
	)
	require.NoError(t, err)
	return newGDriveStorageWithCreator(&driveFileCreator{svc: svc}, folderID)
}

func TestGDriveIntegration_SmallFile(t *testing.T) {
	fake := fakedriveserver.New()
	defer fake.Close()

	s := newFakeDriveStorage(t, fake, "folder-abc")

	fileContent := "fake video content"
	p := filepath.Join(t.TempDir(), "clip.mp4")
	require.NoError(t, os.WriteFile(p, []byte(fileContent), 0644))

	event := &models.Event{
		ID:        "evt-small",
		FilePath:  p,
		Zone:      "FrontGate",
		CameraIP:  "10.0.0.1",
		Timestamp: time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC),
	}

	link, err := s.Save(context.Background(), event)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(link, "https://fake.drive.google.com/"), "expected fake Drive link, got: %s", link)

	uploads := fake.Uploads()
	require.Len(t, uploads, 1)
	assert.Equal(t, []string{"folder-abc"}, uploads[0].Parents)
	assert.True(t, strings.HasPrefix(uploads[0].Name, "20260311T120000_FrontGate_"),
		"unexpected name: %s", uploads[0].Name)
	assert.True(t, strings.HasSuffix(uploads[0].Name, "clip.mp4"),
		"unexpected name: %s", uploads[0].Name)
	assert.Equal(t, fileContent, string(uploads[0].Content))
}

func TestGDriveIntegration_LargeFile(t *testing.T) {
	fake := fakedriveserver.New()
	defer fake.Close()

	s := newFakeDriveStorage(t, fake, "folder-large")

	// Write a file just over the 5 MiB resumable threshold.
	p := filepath.Join(t.TempDir(), "big.mp4")
	require.NoError(t, os.WriteFile(p, make([]byte, gdriveResumableThreshold+1), 0644))

	event := &models.Event{
		ID:        "evt-large",
		FilePath:  p,
		Zone:      "Warehouse",
		CameraIP:  "10.0.0.2",
		Timestamp: time.Now(),
	}

	link, err := s.Save(context.Background(), event)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(link, "https://fake.drive.google.com/"), "expected fake Drive link, got: %s", link)

	uploads := fake.Uploads()
	require.Len(t, uploads, 1)
	assert.Equal(t, []string{"folder-large"}, uploads[0].Parents)
	assert.Len(t, uploads[0].Content, gdriveResumableThreshold+1)
}

func TestGDriveIntegration_UploadError(t *testing.T) {
	fake := fakedriveserver.New()
	defer fake.Close()

	fake.InjectError(500)
	s := newFakeDriveStorage(t, fake, "folder-err")

	p := filepath.Join(t.TempDir(), "clip.mp4")
	require.NoError(t, os.WriteFile(p, []byte("data"), 0644))

	event := &models.Event{
		ID:        "evt-err",
		FilePath:  p,
		Zone:      "Zone1",
		Timestamp: time.Now(),
	}

	_, err := s.Save(context.Background(), event)

	require.Error(t, err)
	assert.ErrorContains(t, err, "gdrive upload")
	assert.Empty(t, fake.Uploads(), "no upload should be recorded on error")
}
