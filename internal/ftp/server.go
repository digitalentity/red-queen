package ftp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"redqueen/internal/config"
	"redqueen/internal/coordinator"
	"redqueen/internal/zone"

	ftpserver "github.com/fclairamb/ftpserverlib"
	"github.com/spf13/afero"
	"go.uber.org/zap"
)

type Server struct {
	logger      *zap.Logger
	config      config.FTPConfig
	coordinator *coordinator.Coordinator
	zoneManager zone.Manager
	server      *ftpserver.FtpServer
}

func NewServer(
	logger *zap.Logger,
	cfg config.FTPConfig,
	coord *coordinator.Coordinator,
	zm zone.Manager,
) *Server {
	return &Server{
		logger:      logger,
		config:      cfg,
		coordinator: coord,
		zoneManager: zm,
	}
}

func (s *Server) Start() error {
	driver := &MainDriver{
		logger:      s.logger,
		config:      s.config,
		coordinator: s.coordinator,
		zoneManager: s.zoneManager,
	}

	s.server = ftpserver.NewFtpServer(driver)

	s.logger.Info("FTP server starting", zap.String("listen_addr", s.config.ListenAddress), zap.Int("port", s.config.Port))
	return s.server.ListenAndServe()
}

func (s *Server) Stop() error {
	if s.server != nil {
		return s.server.Stop()
	}
	return nil
}

// MainDriver implements ftpserver.MainDriver
type MainDriver struct {
	logger      *zap.Logger
	config      config.FTPConfig
	coordinator *coordinator.Coordinator
	zoneManager zone.Manager
}

func (d *MainDriver) GetSettings() (*ftpserver.Settings, error) {
	return &ftpserver.Settings{
		ListenAddr: fmt.Sprintf("%s:%d", d.config.ListenAddress, d.config.Port),
	}, nil
}

func (d *MainDriver) ClientConnected(cc ftpserver.ClientContext) (string, error) {
	ipStr, _, _ := net.SplitHostPort(cc.RemoteAddr().String())
	d.logger.Debug("Client connected", zap.String("ip", ipStr))
	return "Welcome to Red Queen FTP server", nil
}

func (d *MainDriver) ClientDisconnected(cc ftpserver.ClientContext) {
	ipStr, _, _ := net.SplitHostPort(cc.RemoteAddr().String())
	d.logger.Debug("Client disconnected", zap.String("ip", ipStr))
}

func (d *MainDriver) AuthUser(cc ftpserver.ClientContext, user, pass string) (ftpserver.ClientDriver, error) {
	ipStr, _, _ := net.SplitHostPort(cc.RemoteAddr().String())

	// Check credentials
	if user != d.config.User || pass != d.config.Password {
		d.logger.Warn("Invalid credentials", zap.String("ip", ipStr), zap.String("user", user))
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check zone
	zoneName, ok := d.zoneManager.GetZone(ipStr)
	if !ok {
		d.logger.Warn("Unauthorized camera connection attempt", zap.String("ip", ipStr))
		return nil, fmt.Errorf("unauthorized camera")
	}

	// Ensure temp dir exists
	if err := os.MkdirAll(d.config.TempDir, 0755); err != nil {
		return nil, err
	}

	// We use the temp directory as the root for this camera
	baseFs := afero.NewBasePathFs(afero.NewOsFs(), d.config.TempDir)

	d.logger.Debug("User authenticated",
		zap.String("ip", ipStr),
		zap.String("user", user),
		zap.String("zone", zoneName),
	)

	return &ObservedFs{
		Fs:          baseFs,
		logger:      d.logger,
		coordinator: d.coordinator,
		ip:          ipStr,
		zone:        zoneName,
		tempDir:     d.config.TempDir,
	}, nil
}

func (d *MainDriver) GetTLSConfig() (*tls.Config, error) {
	return nil, fmt.Errorf("TLS not supported")
}

// ObservedFs wraps afero.Fs to catch when a file is closed after writing
// It implements ftpserver.ClientDriver because it embeds afero.Fs
type ObservedFs struct {
	afero.Fs
	logger      *zap.Logger
	coordinator *coordinator.Coordinator
	ip          string
	zone        string
	tempDir     string
}

func (fs *ObservedFs) Create(name string) (afero.File, error) {
	return fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func (fs *ObservedFs) Remove(name string) error {
	fs.logger.Debug("Removing file", zap.String("name", name), zap.String("ip", fs.ip))
	return fs.Fs.Remove(name)
}

func (fs *ObservedFs) Rename(oldname, newname string) error {
	fs.logger.Debug("Renaming file",
		zap.String("old", oldname),
		zap.String("new", newname),
		zap.String("ip", fs.ip),
	)
	return fs.Fs.Rename(oldname, newname)
}

func (fs *ObservedFs) Mkdir(name string, perm os.FileMode) error {
	fs.logger.Debug("Ignoring directory creation (flattening enabled)", zap.String("name", name), zap.String("ip", fs.ip))
	return nil
}

func (fs *ObservedFs) MkdirAll(path string, perm os.FileMode) error {
	fs.logger.Debug("Ignoring directory creation (flattening enabled)", zap.String("path", path), zap.String("ip", fs.ip))
	return nil
}

func (fs *ObservedFs) Stat(name string) (os.FileInfo, error) {
	fi, err := fs.Fs.Stat(name)
	if err != nil {
		// If the file/dir doesn't exist on disk, we pretend it's a directory.
		// This satisfies clients that try to CWD or check directories before uploading,
		// while we actually flatten everything into the root temp directory.
		return &fakeFileInfo{name: filepath.Base(name)}, nil
	}
	return fi, nil
}

func (fs *ObservedFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	// We want to ensure unique names in the shared temp directory
	uniqueName := fmt.Sprintf("%d-%s", os.Getpid(), filepath.Base(name))

	// Open/Create the file in the base filesystem
	f, err := fs.Fs.OpenFile(uniqueName, flag, perm)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(fs.tempDir, uniqueName)

	// We only want to trigger processing for files that were opened for writing
	isWrite := flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE) != 0

	fs.logger.Debug("Opening file",
		zap.String("name", name),
		zap.String("unique_name", uniqueName),
		zap.Bool("write", isWrite),
		zap.String("ip", fs.ip),
	)

	return &ObservedFile{
		File:        f,
		fullPath:    fullPath,
		coordinator: fs.coordinator,
		logger:      fs.logger,
		ip:          fs.ip,
		zone:        fs.zone,
		isWrite:     isWrite,
	}, nil
}

type ObservedFile struct {
	afero.File
	fullPath    string
	coordinator *coordinator.Coordinator
	logger      *zap.Logger
	ip          string
	zone        string
	isWrite     bool
}

func (f *ObservedFile) Close() error {
	err := f.File.Close()
	if err == nil && f.isWrite {
		f.logger.Debug("File closed, triggering analysis",
			zap.String("path", f.fullPath),
			zap.String("ip", f.ip),
			zap.String("zone", f.zone),
		)
		// Trigger analysis
		go f.coordinator.Process(context.Background(), f.fullPath, f.ip, f.zone)
	}
	return err
}

type fakeFileInfo struct {
	name string
}

func (f *fakeFileInfo) Name() string       { return f.name }
func (f *fakeFileInfo) Size() int64        { return 0 }
func (f *fakeFileInfo) Mode() os.FileMode  { return os.ModeDir | 0755 }
func (f *fakeFileInfo) ModTime() time.Time { return time.Now() }
func (f *fakeFileInfo) IsDir() bool        { return true }
func (f *fakeFileInfo) Sys() interface{}   { return nil }
