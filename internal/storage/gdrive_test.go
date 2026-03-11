package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/drive/v3"

	"redqueen/internal/models"
)

// mockFileCreator captures the arguments passed to create and returns configurable results.
type mockFileCreator struct {
	capturedMeta    *drive.File
	capturedContent []byte
	capturedSize    int64
	returnLink      string
	returnErr       error
}

func (m *mockFileCreator) create(_ context.Context, file *drive.File, content io.Reader, size int64) (string, error) {
	m.capturedMeta = file
	m.capturedSize = size
	if content != nil {
		m.capturedContent, _ = io.ReadAll(content)
	}
	return m.returnLink, m.returnErr
}

func makeEvent(t *testing.T, dir, content string) *models.Event {
	t.Helper()
	p := filepath.Join(dir, "clip.mp4")
	require.NoError(t, os.WriteFile(p, []byte(content), 0644))
	return &models.Event{
		ID:        "evt-1",
		FilePath:  p,
		Zone:      "FrontGate",
		CameraIP:  "10.0.0.1",
		Timestamp: time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC),
	}
}

func TestGDriveStorage_Type(t *testing.T) {
	s := newGDriveStorageWithCreator(&mockFileCreator{}, "folder-1")
	assert.Equal(t, "google_drive", s.Type())
}

func TestGDriveStorage_Save_Success(t *testing.T) {
	dir := t.TempDir()
	mock := &mockFileCreator{returnLink: "https://drive.google.com/file/d/abc/view"}
	s := newGDriveStorageWithCreator(mock, "folder-1")

	event := makeEvent(t, dir, "fake video data")
	link, err := s.Save(context.Background(), event)

	require.NoError(t, err)
	assert.Equal(t, "https://drive.google.com/file/d/abc/view", link)

	// Metadata
	require.NotNil(t, mock.capturedMeta)
	assert.Equal(t, []string{"folder-1"}, mock.capturedMeta.Parents)
	assert.True(t, strings.HasPrefix(mock.capturedMeta.Name, "20260311T120000_FrontGate_"),
		"name should start with timestamp_zone_, got: %s", mock.capturedMeta.Name)
	assert.True(t, strings.HasSuffix(mock.capturedMeta.Name, "clip.mp4"),
		"name should end with original filename, got: %s", mock.capturedMeta.Name)

	// Content forwarded intact
	assert.Equal(t, "fake video data", string(mock.capturedContent))
}

func TestGDriveStorage_Save_SmallFileUsesInlineUpload(t *testing.T) {
	dir := t.TempDir()
	mock := &mockFileCreator{returnLink: "https://drive.google.com/file/d/abc/view"}
	s := newGDriveStorageWithCreator(mock, "folder-1")

	event := makeEvent(t, dir, "small")
	_, err := s.Save(context.Background(), event)

	require.NoError(t, err)
	assert.Less(t, mock.capturedSize, int64(gdriveResumableThreshold))
}

func TestGDriveStorage_Save_LargeFileUsesResumableUpload(t *testing.T) {
	dir := t.TempDir()

	// Write a file just over the 5 MiB threshold.
	p := filepath.Join(dir, "big.mp4")
	require.NoError(t, os.WriteFile(p, make([]byte, gdriveResumableThreshold+1), 0644))

	mock := &mockFileCreator{returnLink: "https://drive.google.com/file/d/xyz/view"}
	s := newGDriveStorageWithCreator(mock, "folder-1")

	event := &models.Event{
		ID:        "evt-big",
		FilePath:  p,
		Zone:      "Warehouse",
		Timestamp: time.Now(),
	}
	_, err := s.Save(context.Background(), event)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, mock.capturedSize, int64(gdriveResumableThreshold))
}

func TestGDriveStorage_Save_UploadError(t *testing.T) {
	dir := t.TempDir()
	uploadErr := errors.New("drive: quota exceeded")
	mock := &mockFileCreator{returnErr: uploadErr}
	s := newGDriveStorageWithCreator(mock, "folder-1")

	event := makeEvent(t, dir, "data")
	_, err := s.Save(context.Background(), event)

	require.Error(t, err)
	assert.ErrorContains(t, err, "gdrive upload")
	assert.ErrorContains(t, err, "quota exceeded")
}

func TestGDriveStorage_Save_FileNotFound(t *testing.T) {
	mock := &mockFileCreator{}
	s := newGDriveStorageWithCreator(mock, "folder-1")

	event := &models.Event{
		ID:        "evt-missing",
		FilePath:  "/nonexistent/path/clip.mp4",
		Zone:      "Zone1",
		Timestamp: time.Now(),
	}
	_, err := s.Save(context.Background(), event)

	require.Error(t, err)
	// create should never have been called
	assert.Nil(t, mock.capturedMeta)
}
