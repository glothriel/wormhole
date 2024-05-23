package nginx

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

type PortAllocator interface {
	Allocate() (int, error)
	Return(int)
}

type rangePortAllocator struct {
	start int
	end   int
	used  map[int]struct{}
	lock  sync.Mutex
}

func (r *rangePortAllocator) Allocate() (int, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	for i := r.start; i < r.end; i++ {
		if _, ok := r.used[i]; ok {
			continue
		}
		r.used[i] = struct{}{}
		return i, nil
	}
	return 0, errors.New("no ports available")
}

func (r *rangePortAllocator) Return(port int) {
	delete(r.used, port)
}

// validatingRangePortAllocator is the decorator that validates if a port is physically open for listening.
type validatingRangePortAllocator struct {
	child PortAllocator
}

func (v *validatingRangePortAllocator) Allocate() (int, error) {
	for {
		port, err := v.child.Allocate()
		if err != nil {
			return 0, err
		}

		// Check if the port is physically open for listening
		if isPortOpen(port) {
			return port, nil
		} else {
			// If not open, return it and try another
			v.child.Return(port)
		}
	}
}

func (v *validatingRangePortAllocator) Return(port int) {
	v.child.Return(port)
}

// isPortOpen checks if a port is open for listening
func isPortOpen(port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", fmt.Sprint(port)))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func NewRangePortAllocator(start, end int) PortAllocator {
	return &validatingRangePortAllocator{
		child: &rangePortAllocator{
			start: start,
			end:   end,
			used:  make(map[int]struct{}),
		},
	}
}
