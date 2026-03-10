// +build integration

package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullSystemIntegration(t *testing.T) {
	// Create a parent temp directory for all test assets
	testParentDir, err := os.MkdirTemp("", "redqueen-integration-*")
	require.NoError(t, err)
	defer os.RemoveAll(testParentDir)

	// Sub-directories for different concerns
	tmpDir := filepath.Join(testParentDir, "uploads")
	storageDir := filepath.Join(testParentDir, "storage")
	require.NoError(t, os.Mkdir(tmpDir, 0755))
	require.NoError(t, os.Mkdir(storageDir, 0755))

	// 1. Generate a temporary config file from template
	configPath := filepath.Join(testParentDir, "config.yaml")
	tmpl, err := template.ParseFiles("config.test.yaml.tmpl")
	require.NoError(t, err)

	f, err := os.Create(configPath)
	require.NoError(t, err)
	
	err = tmpl.Execute(f, struct {
		TempDir    string
		StorageDir string
	}{
		TempDir:    tmpDir,
		StorageDir: storageDir,
	})
	require.NoError(t, err)
	f.Close()

	// 2. Start Mock Webhook Receiver
	webhookChan := make(chan map[string]interface{}, 1)
	webhookServer := &http.Server{
		Addr: ":9999",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
				webhookChan <- payload
			}
			w.WriteHeader(http.StatusOK)
		}),
	}
	go webhookServer.ListenAndServe()
	defer webhookServer.Close()

	// 3. Start Red Queen in the background
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	binaryPath := filepath.Join(root, "red-queen")

	cmd := exec.Command(binaryPath)
	cmd.Dir = testParentDir // Run from the temp directory
	cmd.Env = append(os.Environ(), 
		"RED_QUEEN_CONFIG="+configPath, 
		"RED_QUEEN_MOCK_THREAT=true",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start())
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	// Wait for services to start
	time.Sleep(2 * time.Second)

	// 4. Create and upload a file via FTP using curl
	testFileName := "camera_clip.mp4"
	testFilePath := filepath.Join(testParentDir, testFileName)
	require.NoError(t, os.WriteFile(testFilePath, []byte("fake video content"), 0644))

	curlCmd := exec.Command("curl", "-s", "--user", "testuser:testpassword", "-T", testFilePath, "ftp://127.0.0.1:2121/")
	out, err := curlCmd.CombinedOutput()
	require.NoError(t, err, string(out))

	// 5. Wait for Webhook
	select {
	case payload := <-webhookChan:
		t.Logf("Received webhook: %+v", payload)
		assert.Equal(t, "TestZone", payload["zone"])
		assert.Equal(t, "127.0.0.1", payload["camera_ip"])
		assert.True(t, payload["is_threat"].(bool))
		
		artifactURL := payload["artifact_url"].(string)
		assert.NotEmpty(t, artifactURL)

		// 6. Verify file in storage
		today := time.Now().Format("2006-01-02")
		expectedDir := filepath.Join(storageDir, today, "TestZone")
		files, err := os.ReadDir(expectedDir)
		require.NoError(t, err)
		assert.NotEmpty(t, files, "Should find at least one file in storage")

		// 7. Verify accessibility via REST API
		apiURL := fmt.Sprintf("http://127.0.0.1:8080%s", artifactURL)
		t.Logf("Checking API at: %s", apiURL)
		
		resp, err := http.Get(apiURL)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		content, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "fake video content", string(content))

	case <-time.After(10 * time.Second):
		t.Fatal("Timed out waiting for webhook")
	}
}
