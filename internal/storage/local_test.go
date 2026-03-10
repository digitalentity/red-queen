package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"redqueen/internal/config"
	"redqueen/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorage_Save(t *testing.T) {
	// Setup temporary directories
	tempBase, err := os.MkdirTemp("", "redqueen-test-storage-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempBase)

	sourceDir := filepath.Join(tempBase, "source")
	destDir := filepath.Join(tempBase, "dest")
	require.NoError(t, os.Mkdir(sourceDir, 0755))
	require.NoError(t, os.Mkdir(destDir, 0755))

	// Create a dummy source file
	sourceFileName := "testfile.txt"
	sourceFilePath := filepath.Join(sourceDir, sourceFileName)
	content := []byte("hello red queen")
	require.NoError(t, os.WriteFile(sourceFilePath, content, 0644))

	// Create the storage provider
	cfg := config.LocalConfig{RootPath: destDir}
	provider := NewLocalStorage(cfg)

	// Create a test event
	now := time.Now()
	event := &models.Event{
		ID:        "test-uuid",
		FilePath:  sourceFilePath,
		Timestamp: now,
		Zone:      "front-gate",
	}

	// Save the event
	resultPath, err := provider.Save(context.Background(), event)
	require.NoError(t, err)

	// Verify the result
	assert.NotEmpty(t, resultPath)
	assert.True(t, filepath.IsAbs(resultPath))

	// Verify file was moved to the correct location
	// Expected: destDir/YYYY-MM-DD/zone/ID_filename
	expectedDir := filepath.Join(destDir, now.Format("2006-01-02"), "front-gate")
	expectedPath := filepath.Join(expectedDir, "test-uuid_"+sourceFileName)
	
	assert.Equal(t, expectedPath, resultPath)
	assert.FileExists(t, expectedPath)
	
	// Verify source file is gone
	assert.NoFileExists(t, sourceFilePath)

	// Verify content
	savedContent, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, content, savedContent)
}
