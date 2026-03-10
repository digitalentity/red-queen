package zone

import "redqueen/internal/config"

type managerImpl struct {
	registry map[string]string
}

// NewManager creates a new Zone Manager from the provided configuration.
func NewManager(zones []config.ZoneConfig) Manager {
	registry := make(map[string]string)
	for _, z := range zones {
		for _, c := range z.Cameras {
			registry[c.IP] = z.Name
		}
	}
	return &managerImpl{registry: registry}
}

func (m *managerImpl) GetZone(ip string) (string, bool) {
	zone, ok := m.registry[ip]
	return zone, ok
}
