package ports

import (
	"github.com/phayes/freeport"
	"k8s.io/apimachinery/pkg/util/rand"
)

// Allocator is used to select port the app should listen on
type Allocator interface {
	GetFreePort() (int, error)
}

// RandomPortFromARangeAllocator returns random number from a range
type RandomPortFromARangeAllocator struct {
	Min, Max int
}

// GetFreePort implements PortAllocator
func (allocator RandomPortFromARangeAllocator) GetFreePort() (int, error) {
	return rand.IntnRange(allocator.Min, allocator.Max), nil
}

// RandomPortAllocator allocates random port
type RandomPortAllocator struct{}

// GetFreePort implements PortAllocator
func (allocator RandomPortAllocator) GetFreePort() (int, error) {
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
