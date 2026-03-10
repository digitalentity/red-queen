package zone

type Manager interface {
	GetZone(ip string) (string, bool)
}
