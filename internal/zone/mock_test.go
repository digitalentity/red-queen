package zone

type MockManager struct {
	Registry map[string]string
}

func (m *MockManager) GetZone(ip string) (string, bool) {
	if m.Registry == nil {
		return "", false
	}
	zone, ok := m.Registry[ip]
	return zone, ok
}
