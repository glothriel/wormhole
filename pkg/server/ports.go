package server

import "github.com/phayes/freeport"

// PortAllocator is used to select port the app should listen on
type PortAllocator interface {
	GetFreePort() (int, error)
}

// RandomPortAllocator allocates random port
type RandomPortAllocator struct{}

// GetFreePort implements PortAllocator
func (allocaor RandomPortAllocator) GetFreePort() (int, error) {
	return freeport.GetFreePort()
}

// PredefinedPortAllocator allows selecting concrete port to allocate
type PredefinedPortAllocator struct {
	ThePort int
}

// GetFreePort implements PortAllocator
func (allocaor PredefinedPortAllocator) GetFreePort() (int, error) {
	return allocaor.ThePort, nil
}
