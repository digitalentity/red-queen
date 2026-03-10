package ftp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"

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
	return nil, nil
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
	// We want to ensure unique names in the shared temp directory
	uniqueName := fmt.Sprintf("%d-%s", os.Getpid(), filepath.Base(name))
	
	// Create the file in the base filesystem (which is d.config.TempDir)
	f, err := fs.Fs.Create(uniqueName)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(fs.tempDir, uniqueName)

	return &ObservedFile{
		File:        f,
		fullPath:    fullPath,
		coordinator: fs.coordinator,
		ip:          fs.ip,
		zone:        fs.zone,
	}, nil
}

type ObservedFile struct {
	afero.File
	fullPath    string
	coordinator *coordinator.Coordinator
	ip          string
	zone        string
}

func (f *ObservedFile) Close() error {
	err := f.File.Close()
	if err == nil {
		// Trigger analysis
		go f.coordinator.Process(context.Background(), f.fullPath, f.ip, f.zone)
	}
	return err
}
