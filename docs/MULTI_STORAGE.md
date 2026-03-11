# Multi-Provider Storage with Google Drive

## Overview

This document describes the design for supporting multiple storage backends simultaneously, including a new Google Drive provider. Artifacts are uploaded to all configured backends concurrently; failure of one does not block the others or halt event processing.

## Goals

- Upload threat artifacts to all configured storage backends in parallel.
- Isolate failures: a single backend failing must not prevent others from saving.
- Add a Google Drive backend as a first-class provider.
- Keep the `storage.Provider` interface unchanged.
- Mirror the fan-out pattern already used for `[]notify.Notifier`.

## Non-Goals

- Serving artifacts stored in Google Drive via the API (Drive's own web links are used instead; see Artifact Serving below).
- Cross-provider deduplication or synchronisation.

---

## Configuration

The current `StorageConfig` uses a single `provider` string selector. This is replaced with a list of provider entries, following the same shape as `[]NotifyConfig`. The project is in active development; all `config.yaml` files are updated directly to the new format — no migration shim is provided.

### Before

```yaml
storage:
  provider: local
  local:
    root_path: /var/red-queen/artifacts
```

### After

```yaml
storage:
  providers:
    - type: local
      local:
        root_path: /var/red-queen/artifacts
    - type: google_drive
      google_drive:
        credentials_file: /etc/red-queen/gdrive-sa.json
        folder_id: "1AbCdEfGhIjKlMnOpQrStUvWx"
```

### New Config Structs (`internal/config/config.go`)

```go
type StorageConfig struct {
    Providers []StorageProviderConfig `mapstructure:"providers"`
}

type StorageProviderConfig struct {
    Type        string       `mapstructure:"type"`         // "local" | "google_drive"
    Local       LocalConfig  `mapstructure:"local"`
    GoogleDrive GDriveConfig `mapstructure:"google_drive"`
}

type GDriveConfig struct {
    // CredentialsFile is the path to a Google service account JSON key file.
    CredentialsFile string `mapstructure:"credentials_file"`
    // FolderID is the Drive folder that uploaded artifacts are placed in.
    // The folder must be shared with the service account as Editor.
    FolderID string `mapstructure:"folder_id"`
}
```

`LocalConfig` is otherwise unchanged. `S3Config` and the old top-level `Provider string` field are removed.

---

## New Types

### `MultiProvider` (`internal/storage/multi.go`)

`MultiProvider` implements `storage.Provider` and fans out `Save` calls to all backing providers concurrently. It is only constructed when two or more providers are configured; with a single provider, `app.go` uses it directly.

```go
// MultiProvider uploads to all providers concurrently.
// It returns the first successful URL.
// It returns an error only if every provider fails.
type MultiProvider struct {
    providers []Provider
    logger    *zap.Logger
}

func NewMultiProvider(providers []Provider, logger *zap.Logger) *MultiProvider

func (m *MultiProvider) Type() string { return "multi" }

func (m *MultiProvider) Save(ctx context.Context, event *models.Event) (string, error) {
    type result struct {
        url string
        err error
        typ string
    }

    results := make([]result, len(m.providers))
    var wg sync.WaitGroup
    for i, p := range m.providers {
        i, p := i, p
        wg.Add(1)
        go func() {
            defer wg.Done()
            url, err := p.Save(ctx, event)
            results[i] = result{url: url, err: err, typ: p.Type()}
        }()
    }
    wg.Wait()

    var firstURL string
    var errs []string
    for _, r := range results {
        if r.err != nil {
            m.logger.Error("Storage provider failed",
                zap.String("type", r.typ), zap.Error(r.err))
            errs = append(errs, fmt.Sprintf("%s: %s", r.typ, r.err))
        } else {
            m.logger.Info("Storage provider succeeded",
                zap.String("type", r.typ), zap.String("url", r.url))
            if firstURL == "" {
                firstURL = r.url
            }
        }
    }

    if firstURL == "" {
        return "", fmt.Errorf("all storage providers failed: %s", strings.Join(errs, "; "))
    }
    return firstURL, nil
}
```

**URL selection**: The URL of the first provider to succeed (in config order) is returned to the coordinator and passed on to notifiers. All other URLs are logged. This keeps notifications simple — they expect a single string.

**Metrics**: Each backing provider records `StorageOperations` with its own `Type()` label inside its own `Save` implementation. `MultiProvider` itself does not record additional metrics, avoiding double-counting.

### `GDriveStorage` (`internal/storage/gdrive.go`)

#### Authentication and scope

Authentication uses a **service account JSON key file** — appropriate for server-to-server use without interactive OAuth. The required OAuth scope is `drive.file` (`DriveFileScope`), which is the least-privilege choice: it permits the service account to create files in any folder that has been shared with it as Editor. The folder does not need to have been created by the service account. Full drive access (`DriveScope`) must not be used.

Required dependencies:

```
google.golang.org/api      (drive/v3)
golang.org/x/oauth2/google
```

#### File visibility

Uploaded artifacts are **private**. Files inherit the sharing settings of the parent folder and are accessible only to accounts that the folder has been explicitly shared with. No `Permissions.Create` call is made after upload. The `webViewLink` returned by the Drive API requires the recipient to be authenticated and to have folder access — it is not a public link. Operators must be aware that Telegram and webhook notifications will include this link, but recipients without folder access will not be able to open it.

#### Upload method

`Save` stats the file before uploading. Files larger than 5 MiB use `ResumableMedia`; smaller files use `Media`. The threshold is hardcoded.

#### Implementation

`GDriveStorage` holds a narrow `fileCreator` interface rather than a `*drive.Service` directly. This keeps the type testable without hitting the real API — unit tests inject a `mockFileCreator`; integration tests inject a `driveFileCreator` pointed at `fakedriveserver`.

```go
// fileCreator is a narrow interface over the Drive Files API to allow test injection.
type fileCreator interface {
    create(ctx context.Context, file *drive.File, content io.Reader, size int64) (webViewLink string, err error)
}

type GDriveStorage struct {
    creator  fileCreator
    folderID string
}

func NewGDriveStorage(ctx context.Context, cfg config.GDriveConfig) (*GDriveStorage, error) {
    data, err := os.ReadFile(cfg.CredentialsFile)
    ...
    creds, err := google.CredentialsFromJSON(ctx, data, drive.DriveFileScope)
    ...
    svc, err := drive.NewService(ctx, option.WithCredentials(creds))
    ...
    return &GDriveStorage{creator: &driveFileCreator{svc: svc}, folderID: cfg.FolderID}, nil
}

func (g *GDriveStorage) Type() string { return "google_drive" }

const gdriveResumableThreshold = 5 * 1024 * 1024 // 5 MiB

func (g *GDriveStorage) Save(ctx context.Context, event *models.Event) (string, error) {
    info, err := os.Stat(event.FilePath)
    ...
    f, err := os.Open(event.FilePath)
    ...
    meta := &drive.File{
        Name:    fmt.Sprintf("%s_%s_%s", event.Timestamp.Format("20060102T150405"), event.Zone, filepath.Base(event.FilePath)),
        Parents: []string{g.folderID},
    }
    link, err := g.creator.create(ctx, meta, f, info.Size())
    ...
    return link, nil
}

// driveFileCreator is the production fileCreator backed by a real drive.Service.
// It selects multipart vs resumable upload based on gdriveResumableThreshold.
// ResumableMedia requires io.ReaderAt; *os.File satisfies this in production.
type driveFileCreator struct{ svc *drive.Service }

func (c *driveFileCreator) create(ctx context.Context, file *drive.File, content io.Reader, size int64) (string, error) {
    call := c.svc.Files.Create(file).Fields("id,webViewLink").Context(ctx)
    if size >= gdriveResumableThreshold {
        ra := content.(io.ReaderAt) // *os.File always satisfies this
        call = call.ResumableMedia(ctx, ra, size, "application/octet-stream")
    } else {
        call = call.Media(content)
    }
    created, err := call.Do()
    ...
    return created.WebViewLink, nil
}
```

**File naming**: `{YYYYMMDDTHHMMSS}_{zone}_{original_filename}` — deterministic, sortable, and includes enough context to identify the artifact without opening it.

---

## Wiring (`internal/app/app.go`)

The `switch cfg.Storage.Provider` block is replaced:

```go
// 3. Initialize Storage
var providers []storage.Provider
artifactHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "Artifact serving not available", http.StatusNotFound)
})

for _, pcfg := range cfg.Storage.Providers {
    switch pcfg.Type {
    case "local":
        p := storage.NewLocalStorage(pcfg.Local)
        providers = append(providers, p)
        // The first local provider wins for HTTP artifact serving.
        if _, isDefault := artifactHandler.(http.HandlerFunc); isDefault {
            artifactHandler = http.FileServer(http.Dir(pcfg.Local.RootPath))
        }
        logger.Info("Storage: local enabled", zap.String("root_path", pcfg.Local.RootPath))
    case "google_drive":
        p, err := storage.NewGDriveStorage(ctx, pcfg.GoogleDrive)
        if err != nil {
            cancel()
            return nil, fmt.Errorf("failed to init google_drive storage: %w", err)
        }
        providers = append(providers, p)
        logger.Info("Storage: google_drive enabled",
            zap.String("folder_id", pcfg.GoogleDrive.FolderID))
    default:
        logger.Warn("Unknown storage provider type, skipping",
            zap.String("type", pcfg.Type))
    }
}

var storageProvider storage.Provider
switch len(providers) {
case 0:
    logger.Warn("No storage providers configured, using mock")
    storageProvider = &storage.MockProvider{}
case 1:
    storageProvider = providers[0]
default:
    storageProvider = storage.NewMultiProvider(providers, logger)
}
```

The coordinator constructor call and all downstream code remain unchanged.

---

## Artifact Serving

The REST API endpoint `/artifacts/...` **requires a local storage provider** to be configured. It is backed by an `http.FileServer` rooted at the first local provider's `root_path`. If no local provider is present, the endpoint returns 404.

Deployments that use only Google Drive (or other remote providers) do not have artifacts accessible via the API. Notifications will include the Drive `webViewLink` instead, subject to the access restrictions described above.

A potential improvement — issuing a `302` redirect to the provider's own URL for remote backends — is tracked in `FUTURE_WORK.md`.

---

## File Layout

```
internal/storage/
  storage.go                    interface — unchanged
  local.go                      unchanged
  mock.go                       unchanged
  multi.go                      NEW — MultiProvider fan-out
  gdrive.go                     NEW — Google Drive provider
  local_test.go                 unchanged
  multi_test.go                 NEW — table-driven tests covering partial and total failure
  gdrive_test.go                NEW — unit tests using mockFileCreator
  gdrive_integration_test.go    NEW — integration tests using fakedriveserver (build tag: integration)
  fakedriveserver/
    server.go                   NEW — self-contained fake Drive HTTP server for integration tests
```

---

## Error Handling and Failure Modes

| Scenario | Behaviour |
|---|---|
| One of N providers fails | Error logged with provider type; other providers continue; first successful URL returned |
| All providers fail | `MultiProvider.Save` returns an error; coordinator logs it and proceeds to notify with an empty URL (existing behaviour) |
| Invalid provider config (empty `credentials_file`, `folder_id`, or `root_path`) | Caught by `Config.Validate()` at startup with a clear field-level error message |
| Drive credentials file missing or unreadable | Fatal at startup — `NewGDriveStorage` returns an error, `app.New` propagates it |
| Drive API transient error | Returned as an error from `GDriveStorage.Save`; coordinator does **not** retry storage (retries apply to ML analysis only) |
| Context cancelled during upload | Drive client and local copy loop both respect `ctx`; upload is abandoned cleanly |
| No local provider configured | `/artifacts/...` endpoint returns 404; artifact links in notifications point to provider URLs |
