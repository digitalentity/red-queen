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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullSystemIntegration(t *testing.T) {
	// Create isolated temporary directories for this test run
	tmpDir, err := os.MkdirTemp("", "rq-test-tmp-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storageDir, err := os.MkdirTemp("", "rq-test-storage-*")
	require.NoError(t, err)
	defer os.RemoveAll(storageDir)

	// 1. Start Mock Webhook Receiver
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

	// 2. Start Red Queen in the background
	// We override temp and storage dirs via environment variables
	cmd := exec.Command("./red-queen")
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), 
		"RED_QUEEN_CONFIG=config.test.yaml", 
		"RED_QUEEN_MOCK_THREAT=true",
		"RED_QUEEN_FTP_TEMP_DIR="+tmpDir,
		"RED_QUEEN_STORAGE_LOCAL_ROOT_PATH="+storageDir,
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

	// 3. Upload a file via FTP using curl
	testFile := "camera_clip.mp4"
	require.NoError(t, os.WriteFile(testFile, []byte("fake video content"), 0644))
	defer os.Remove(testFile)

	curlCmd := exec.Command("curl", "-s", "--user", "testuser:testpassword", "-T", testFile, "ftp://127.0.0.1:2121/")
	out, err := curlCmd.CombinedOutput()
	require.NoError(t, err, string(out))

	// 4. Wait for Webhook
	select {
	case payload := <-webhookChan:
		t.Logf("Received webhook: %+v", payload)
		assert.Equal(t, "TestZone", payload["zone"])
		assert.Equal(t, "127.0.0.1", payload["camera_ip"])
		assert.True(t, payload["is_threat"].(bool))
		
		artifactURL := payload["artifact_url"].(string)
		assert.NotEmpty(t, artifactURL)

		// 5. Verify file in storage (using the guaranteed temp storageDir)
		today := time.Now().Format("2006-01-02")
		expectedDir := filepath.Join(storageDir, today, "TestZone")
		files, err := os.ReadDir(expectedDir)
		require.NoError(t, err)
		assert.NotEmpty(t, files, "Should find at least one file in storage")

		// 6. Verify accessibility via REST API
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
