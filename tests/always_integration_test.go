//go:build integration

package tests

import (
	"encoding/json"
	"fmt"
	"net"
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

func TestAlwaysIntegration(t *testing.T) {
	// 0. Find free ports
	getFreePort := func() int {
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		l, err := net.ListenTCP("tcp", addr)
		require.NoError(t, err)
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port
	}

	ftpPort := getFreePort()
	apiPort := getFreePort()
	webhookPort := getFreePort()
	telegramPort := getFreePort()

	testParentDir, err := os.MkdirTemp("", "redqueen-always-*")
	require.NoError(t, err)
	defer os.RemoveAll(testParentDir)

	tmpDir := filepath.Join(testParentDir, "uploads")
	storageDir := filepath.Join(testParentDir, "storage")
	require.NoError(t, os.Mkdir(tmpDir, 0755))
	require.NoError(t, os.Mkdir(storageDir, 0755))

	// 1. Generate config with AlwaysStore=true and WebhookCondition=always
	configPath := filepath.Join(testParentDir, "config.yaml")
	tmpl, err := template.ParseFiles("config.test.yaml.tmpl")
	require.NoError(t, err)

	f, err := os.Create(configPath)
	require.NoError(t, err)
	
	err = tmpl.Execute(f, struct {
		TempDir          string
		StorageDir       string
		FTPPort          int
		APIPort          int
		WebhookPort      int
		TelegramPort     int
		AlwaysStore      bool
		WebhookCondition string
	}{
		TempDir:          tmpDir,
		StorageDir:       storageDir,
		FTPPort:          ftpPort,
		APIPort:          apiPort,
		WebhookPort:      webhookPort,
		TelegramPort:     telegramPort,
		AlwaysStore:      true,
		WebhookCondition: "always",
	})
	require.NoError(t, err)
	f.Close()

	// 2. Start Mock Webhook Receiver
	webhookChan := make(chan map[string]interface{}, 1)
	webhookServer := &http.Server{
		Addr: fmt.Sprintf(":%d", webhookPort),
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

	// 3. Start Red Queen (NOT as a threat)
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	binaryPath := filepath.Join(root, "red-queen")

	cmd := exec.Command(binaryPath)
	cmd.Dir = testParentDir
	cmd.Env = append(os.Environ(), 
		"RED_QUEEN_CONFIG="+configPath, 
		"RED_QUEEN_MOCK_THREAT=false", // No threat!
	)
	require.NoError(t, cmd.Start())
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	// Wait for readiness
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", apiPort)
	require.Eventually(t, func() bool {
		resp, err := http.Get(healthURL)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 15*time.Second, 100*time.Millisecond)

	// 4. Upload file
	testFileName := "normal_event.jpg"
	testFilePath := filepath.Join(testParentDir, testFileName)
	require.NoError(t, os.WriteFile(testFilePath, []byte("normal content"), 0644))

	curlCmd := exec.Command("curl", "-s", "--user", "testuser:testpassword", "-T", testFilePath, fmt.Sprintf("ftp://127.0.0.1:%d/", ftpPort))
	require.NoError(t, curlCmd.Run())

	// 5. Verify Webhook (sent because condition=always)
	select {
	case payload := <-webhookChan:
		assert.False(t, payload["is_threat"].(bool))
		assert.NotEmpty(t, payload["artifact_url"])
		
		// 6. Verify file in storage (stored because always_store=true)
		today := time.Now().Format("2006-01-02")
		expectedDir := filepath.Join(storageDir, today, "TestZone")
		require.Eventually(t, func() bool {
			files, _ := os.ReadDir(expectedDir)
			return len(files) > 0
		}, 5*time.Second, 100*time.Millisecond)

	case <-time.After(15 * time.Second):
		t.Fatal("Timed out waiting for webhook")
	}
}
