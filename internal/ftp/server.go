package ftp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"redqueen/internal/config"
	"redqueen/internal/coordinator"
	"redqueen/internal/zone"

	ftpserver "github.com/fclairamb/ftpserverlib"
	"github.com/google/uuid"
	"github.com/spf13/afero"
	"go.uber.org/zap"
)

type Server struct {
	ctx         context.Context
	logger      *zap.Logger
	config      config.FTPConfig
	coordinator *coordinator.Coordinator
	zoneManager zone.Manager
	server      *ftpserver.FtpServer
}

func NewServer(
	ctx context.Context,
	logger *zap.Logger,
	cfg config.FTPConfig,
	coord *coordinator.Coordinator,
	zm zone.Manager,
) *Server {
	return &Server{
		ctx:         ctx,
		logger:      logger,
		config:      cfg,
		coordinator: coord,
		zoneManager: zm,
	}
}

func (s *Server) Start() error {
	driver := &MainDriver{
		ctx:         s.ctx,
		logger:      s.logger,
		config:      s.config,
		coordinator: s.coordinator,
		zoneManager: s.zoneManager,
		registries:  make(map[string]*VirtualRegistry),
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

// VirtualRegistry maps virtual paths to physical filenames.
type VirtualRegistry struct {
	mu       sync.RWMutex
	mappings map[string]string // virtual path -> physical unique filename
}

func NewVirtualRegistry() *VirtualRegistry {
	return &VirtualRegistry{mappings: make(map[string]string)}
}

// MainDriver implements ftpserver.MainDriver
type MainDriver struct {
	ctx          context.Context
	logger       *zap.Logger
	config       config.FTPConfig
	coordinator  *coordinator.Coordinator
	zoneManager  zone.Manager
	registries   map[string]*VirtualRegistry
	registriesMu sync.Mutex
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

	// Get or create registry for this IP
	d.registriesMu.Lock()
	registry, exists := d.registries[ipStr]
	if !exists {
		registry = NewVirtualRegistry()
		d.registries[ipStr] = registry
	}
	d.registriesMu.Unlock()

	// We use the temp directory as the root for this camera
	baseFs := afero.NewBasePathFs(afero.NewOsFs(), d.config.TempDir)

	d.logger.Debug("User authenticated",
		zap.String("ip", ipStr),
		zap.String("user", user),
		zap.String("zone", zoneName),
	)

	return &ObservedFs{
		Fs:          baseFs,
		ctx:         d.ctx,
		logger:      d.logger,
		coordinator: d.coordinator,
		ip:          ipStr,
		zone:        zoneName,
		tempDir:     d.config.TempDir,
		registry:    registry,
	}, nil
}

func (d *MainDriver) GetTLSConfig() (*tls.Config, error) {
	return nil, fmt.Errorf("TLS not supported")
}

// ObservedFs wraps afero.Fs to catch when a file is closed after writing
type ObservedFs struct {
	afero.Fs
	ctx         context.Context
	logger      *zap.Logger
	coordinator *coordinator.Coordinator
	ip          string
	zone        string
	tempDir     string
	registry    *VirtualRegistry
}

func (fs *ObservedFs) cleanPath(name string) string {
	return filepath.Clean("/" + name)
}

func (fs *ObservedFs) Create(name string) (afero.File, error) {
	return fs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func (fs *ObservedFs) Remove(name string) error {
	name = fs.cleanPath(name)
	fs.registry.mu.Lock()
	physical, ok := fs.registry.mappings[name]
	if ok {
		delete(fs.registry.mappings, name)
	}
	fs.registry.mu.Unlock()

	if !ok {
		return os.ErrNotExist
	}

	fs.logger.Debug("Removing file", zap.String("name", name), zap.String("physical", physical), zap.String("ip", fs.ip))
	return fs.Fs.Remove(physical)
}

func (fs *ObservedFs) Rename(oldname, newname string) error {
	oldname = fs.cleanPath(oldname)
	newname = fs.cleanPath(newname)

	fs.registry.mu.Lock()
	defer fs.registry.mu.Unlock()

	physical, ok := fs.registry.mappings[oldname]
	if !ok {
		return os.ErrNotExist
	}

	// Update mapping
	delete(fs.registry.mappings, oldname)
	fs.registry.mappings[newname] = physical

	fs.logger.Debug("Renaming virtual file",
		zap.String("old", oldname),
		zap.String("new", newname),
		zap.String("physical", physical),
		zap.String("ip", fs.ip),
	)
	return nil
}

func (fs *ObservedFs) Mkdir(name string, perm os.FileMode) error {
	fs.logger.Debug("Virtual directory creation", zap.String("name", name))
	return nil // Directories are purely virtual
}

func (fs *ObservedFs) MkdirAll(path string, perm os.FileMode) error {
	fs.logger.Debug("Virtual directory creation", zap.String("path", path))
	return nil
}

func (fs *ObservedFs) Stat(name string) (os.FileInfo, error) {
	name = fs.cleanPath(name)
	if name == "/" || name == "." {
		return &fakeFileInfo{name: "/", isDir: true}, nil
	}

	fs.registry.mu.RLock()
	physical, ok := fs.registry.mappings[name]
	isDir := false
	if !ok {
		// Check if it's a virtual directory prefix
		prefix := name + "/"
		for v := range fs.registry.mappings {
			if strings.HasPrefix(v, prefix) {
				isDir = true
				break
			}
		}
	}
	fs.registry.mu.RUnlock()

	if ok {
		fi, err := fs.Fs.Stat(physical)
		if err != nil && os.IsNotExist(err) {
			// Lazy pruning of stale mappings
			fs.registry.mu.Lock()
			delete(fs.registry.mappings, name)
			fs.registry.mu.Unlock()
			return nil, err
		}
		if err == nil {
			// Return file info but with the virtual name
			return &fakeFileInfo{
				name:    filepath.Base(name),
				size:    fi.Size(),
				modTime: fi.ModTime(),
				isDir:   false,
			}, nil
		}
		return nil, err
	}

	if isDir {
		return &fakeFileInfo{name: filepath.Base(name), isDir: true}, nil
	}

	return nil, os.ErrNotExist
}

func (fs *ObservedFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	name = fs.cleanPath(name)

	fs.registry.mu.Lock()
	physical, ok := fs.registry.mappings[name]
	if !ok {
		// Only create a new mapping if we are writing
		isCreate := flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE) != 0
		if isCreate {
			physical = fmt.Sprintf("%s-%s-%s", fs.ip, uuid.New().String(), filepath.Base(name))
			fs.registry.mappings[name] = physical
		} else {
			fs.registry.mu.Unlock()
			return nil, os.ErrNotExist
		}
	}
	fs.registry.mu.Unlock()

	// Open/Create the physical file
	f, err := fs.Fs.OpenFile(physical, flag, perm)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(fs.tempDir, physical)
	isWrite := flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE) != 0

	fs.logger.Debug("Opening file",
		zap.String("virtual_name", name),
		zap.String("physical_name", physical),
		zap.Bool("write", isWrite),
		zap.String("ip", fs.ip),
	)

	return &ObservedFile{
		File:        f,
		ctx:         fs.ctx,
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
	ctx         context.Context
	fullPath    string
	coordinator *coordinator.Coordinator
	logger      *zap.Logger
	ip          string
	zone        string
	isWrite     bool
	once        sync.Once
}

func (f *ObservedFile) Close() error {
	err := f.File.Close()
	if err == nil && f.isWrite {
		f.once.Do(func() {
			f.logger.Debug("File closed, triggering analysis",
				zap.String("path", f.fullPath),
				zap.String("ip", f.ip),
				zap.String("zone", f.zone),
			)
			// Trigger analysis
			go f.coordinator.Process(f.ctx, f.fullPath, f.ip, f.zone)
		})
	}
	return err
}

type fakeFileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

func (f *fakeFileInfo) Name() string       { return f.name }
func (f *fakeFileInfo) Size() int64        { return f.size }
func (f *fakeFileInfo) Mode() os.FileMode  { 
	if f.isDir {
		return os.ModeDir | 0755
	}
	return 0644
}
func (f *fakeFileInfo) ModTime() time.Time { return f.modTime }
func (f *fakeFileInfo) IsDir() bool        { return f.isDir }
func (f *fakeFileInfo) Sys() interface{}   { return nil }
