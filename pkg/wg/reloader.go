package wg

// WireguardConfigReloader is an interface for updating Wireguard configuration
type WireguardConfigReloader interface {
	Update(Config) error
}
