package ftp

import (
	"context"
	"os"
	"testing"

	"redqueen/internal/coordinator"
	"redqueen/internal/ml"
	"redqueen/internal/zone"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestObservedFs_VirtualMapping(t *testing.T) {
	logger := zap.NewNop()
	err := os.MkdirAll("test_temp", 0755)
	require.NoError(t, err)
	defer os.RemoveAll("test_temp")

	baseFs := afero.NewMemMapFs()
	registry := NewVirtualRegistry()
	
	// Properly initialize coordinator to avoid panics
	analyzer := &ml.MockAnalyzer{}
	coord := coordinator.NewCoordinator(logger, analyzer, nil, nil, coordinator.CoordinatorConfig{Concurrency: 1})

	fs := &ObservedFs{
		Fs:          baseFs,
		ctx:         context.Background(),
		logger:      logger,
		tempDir:     "test_temp",
		ip:          "1.2.3.4",
		registry:    registry,
		coordinator: coord,
	}

	// 1. Create a virtual file in a subfolder
	virtualPath := "/backyard/cam1/motion.mp4"
	f, err := fs.OpenFile(virtualPath, os.O_RDWR|os.O_CREATE, 0666)
	require.NoError(t, err)
	
	content := []byte("video data")
	_, err = f.Write(content)
	require.NoError(t, err)
	f.Close()

	// 2. Verify visibility via Stat
	fi, err := fs.Stat(virtualPath)
	require.NoError(t, err)
	assert.Equal(t, "motion.mp4", fi.Name())
	assert.Equal(t, int64(len(content)), fi.Size())
	assert.False(t, fi.IsDir())

	// 3. Verify virtual directory visibility
	fi, err = fs.Stat("/backyard")
	require.NoError(t, err)
	assert.True(t, fi.IsDir())

	fi, err = fs.Stat("/backyard/cam1")
	require.NoError(t, err)
	assert.True(t, fi.IsDir())

	// 4. Verify physical file is flattened in root
	files, err := afero.ReadDir(baseFs, "/")
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0].Name(), "1.2.3.4")
	assert.Contains(t, files[0].Name(), "motion.mp4")

	// 5. Deletion
	err = fs.Remove(virtualPath)
	require.NoError(t, err)
	
	_, err = fs.Stat(virtualPath)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	// Mapping should be gone
	registry.mu.RLock()
	assert.Empty(t, registry.mappings)
	registry.mu.RUnlock()
}

func TestMainDriver_RegistryIsolation(t *testing.T) {
	logger := zap.NewNop()
	zm := zone.NewManager(nil)
	coord := &coordinator.Coordinator{} // Empty mock - fine as long as Process isn't called

	driver := &MainDriver{
		logger:      logger,
		coordinator: coord,
		zoneManager: zm,
		registries:  make(map[string]*VirtualRegistry),
	}

	// Two IPs get different registries
	reg1 := driver.getOrCreateRegistry("1.1.1.1")
	reg2 := driver.getOrCreateRegistry("2.2.2.2")

	assert.NotNil(t, reg1)
	assert.NotNil(t, reg2)
	assert.NotSame(t, reg1, reg2)

	// Same IP gets same registry
	reg1Again := driver.getOrCreateRegistry("1.1.1.1")
	assert.Same(t, reg1, reg1Again)
}
