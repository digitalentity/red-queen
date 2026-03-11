// Package fakedriveserver provides a self-contained fake Google Drive HTTP server
// for use in integration tests. It implements only the upload surface of the Drive
// Files API (multipart and resumable) that GDriveStorage exercises.
package fakedriveserver

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
)

// Upload records a single file upload received by the fake server.
type Upload struct {
	// Name is the file name from the Drive metadata.
	Name string
	// Parents is the list of parent folder IDs from the Drive metadata.
	Parents []string
	// Content is the raw file bytes received.
	Content []byte
}

// Server is a fake Google Drive HTTP server backed by httptest.Server.
type Server struct {
	srv *httptest.Server

	mu       sync.Mutex
	uploads  []Upload
	sessions map[string]fileMeta // session ID → pending metadata

	nextError atomic.Int32 // non-zero means inject this HTTP status on next upload
	uploadSeq atomic.Int64 // monotonic counter for generating IDs
}

// fileMeta holds the Drive file metadata extracted from an upload request.
type fileMeta struct {
	Name    string   `json:"name"`
	Parents []string `json:"parents"`
}

// New creates and starts a fake Drive server.
func New() *Server {
	s := &Server{
		sessions: make(map[string]fileMeta),
	}
	s.nextError.Store(0)
	s.srv = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

// URL returns the base URL of the fake server to pass to option.WithEndpoint.
func (s *Server) URL() string { return s.srv.URL }

// Client returns an *http.Client that trusts the fake server.
func (s *Server) Client() *http.Client { return s.srv.Client() }

// Close shuts down the fake server.
func (s *Server) Close() { s.srv.Close() }

// Uploads returns a copy of all uploads recorded by the server.
func (s *Server) Uploads() []Upload {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Upload, len(s.uploads))
	copy(out, s.uploads)
	return out
}

// InjectError causes the next upload request to return the given HTTP status code.
func (s *Server) InjectError(code int) {
	s.nextError.Store(int32(code))
}

// handle routes incoming requests to the appropriate handler.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	uploadType := r.URL.Query().Get("uploadType")
	switch {
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/upload/drive/v3/files") && uploadType == "multipart":
		s.handleMultipart(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/upload/drive/v3/files") && uploadType == "resumable":
		s.handleResumableInit(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/upload/session/"):
		s.handleResumableUpload(w, r)
	default:
		http.Error(w, fmt.Sprintf("fakedriveserver: unhandled %s %s?uploadType=%s", r.Method, r.URL.Path, uploadType), http.StatusNotFound)
	}
}

// handleMultipart handles POST /upload/drive/v3/files?uploadType=multipart.
func (s *Server) handleMultipart(w http.ResponseWriter, r *http.Request) {
	if code := s.consumeInjectedError(); code != 0 {
		http.Error(w, "injected error", code)
		return
	}

	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		http.Error(w, "fakedriveserver: expected multipart/related body", http.StatusBadRequest)
		return
	}

	mr := multipart.NewReader(r.Body, params["boundary"])

	// Part 1: JSON metadata.
	metaPart, err := mr.NextPart()
	if err != nil {
		http.Error(w, "fakedriveserver: missing metadata part: "+err.Error(), http.StatusBadRequest)
		return
	}
	var meta fileMeta
	if err := json.NewDecoder(metaPart).Decode(&meta); err != nil {
		http.Error(w, "fakedriveserver: bad metadata JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Part 2: file content.
	contentPart, err := mr.NextPart()
	if err != nil {
		http.Error(w, "fakedriveserver: missing content part: "+err.Error(), http.StatusBadRequest)
		return
	}
	content, err := io.ReadAll(contentPart)
	if err != nil {
		http.Error(w, "fakedriveserver: reading content: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.recordAndRespond(w, meta, content)
}

// handleResumableInit handles POST /resumable/upload/drive/v3/files?uploadType=resumable.
// It stores the metadata and returns a Location header pointing to the upload session URL.
func (s *Server) handleResumableInit(w http.ResponseWriter, r *http.Request) {
	if code := s.consumeInjectedError(); code != 0 {
		http.Error(w, "injected error", code)
		return
	}

	var meta fileMeta
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		http.Error(w, "fakedriveserver: bad metadata JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	sessionID := fmt.Sprintf("session-%d", s.uploadSeq.Add(1))

	s.mu.Lock()
	s.sessions[sessionID] = meta
	s.mu.Unlock()

	w.Header().Set("Location", s.srv.URL+"/upload/session/"+sessionID)
	w.WriteHeader(http.StatusOK)
}

// handleResumableUpload handles POST /upload/session/{sessionID}.
func (s *Server) handleResumableUpload(w http.ResponseWriter, r *http.Request) {
	if code := s.consumeInjectedError(); code != 0 {
		http.Error(w, "injected error", code)
		return
	}

	sessionID := strings.TrimPrefix(r.URL.Path, "/upload/session/")

	s.mu.Lock()
	meta, ok := s.sessions[sessionID]
	if ok {
		delete(s.sessions, sessionID)
	}
	s.mu.Unlock()

	if !ok {
		http.Error(w, "fakedriveserver: unknown session: "+sessionID, http.StatusNotFound)
		return
	}

	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "fakedriveserver: reading content: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.recordAndRespond(w, meta, content)
}

// recordAndRespond appends the upload to the log and writes the Drive File JSON response.
func (s *Server) recordAndRespond(w http.ResponseWriter, meta fileMeta, content []byte) {
	id := fmt.Sprintf("fake-id-%d", s.uploadSeq.Add(1))
	link := fmt.Sprintf("https://fake.drive.google.com/file/d/%s/view", id)

	s.mu.Lock()
	s.uploads = append(s.uploads, Upload{
		Name:    meta.Name,
		Parents: meta.Parents,
		Content: content,
	})
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":          id,
		"webViewLink": link,
	})
}

func (s *Server) consumeInjectedError() int {
	return int(s.nextError.Swap(0))
}
