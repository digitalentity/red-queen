# Future Work

This file tracks known limitations and improvement candidates that are out of scope for their originating feature but worth addressing later.

## Artifact serving for non-local storage providers

**Context**: The REST API endpoint `/artifacts/...` is backed by an `http.FileServer` and only works when a local storage provider is configured. Deployments that use only remote providers (e.g. Google Drive) receive a 404 from this endpoint and must rely on provider-specific links in notifications.

**Improvement**: For remote providers (e.g. Google Drive), the API could issue a `302 Found` redirect to the provider's own URL (e.g. Drive's `webViewLink`) rather than attempting to proxy or serve the content directly. This would keep the API server stateless with respect to remote storage while still making `/artifacts/{id}` a stable, provider-agnostic entry point. A possible design: `storage.Provider` gains an optional `ArtifactURL(id string) (string, bool)` method (or a separate interface); the API handler checks for a redirect URL before falling back to local file serving.

**Introduced by**: `docs/MULTI_STORAGE.md` (multi-provider storage design)
