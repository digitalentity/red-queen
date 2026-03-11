package zone

import (
	"testing"
	"redqueen/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestZoneManager(t *testing.T) {
	cfg := []config.ZoneConfig{
		{
			Name: "Front",
			Cameras: []config.CameraConfig{
				{IP: "1.1.1.1"},
				{IP: "1.1.1.2"},
			},
		},
		{
			Name: "Back",
			Cameras: []config.CameraConfig{
				{IP: "2.2.2.2"},
			},
		},
	}

	m := NewManager(cfg)

	// Test known IPs
	z, ok := m.GetZone("1.1.1.1")
	assert.True(t, ok)
	assert.Equal(t, "Front", z)

	z, ok = m.GetZone("2.2.2.2")
	assert.True(t, ok)
	assert.Equal(t, "Back", z)

	// Test unknown IP
	z, ok = m.GetZone("3.3.3.3")
	assert.False(t, ok)
	assert.Empty(t, z)
}
