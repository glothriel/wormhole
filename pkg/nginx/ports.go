package nginx

import (
	"errors"
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

func NewRangePortAllocator(start, end int) PortAllocator {
	return &rangePortAllocator{
		start: start,
		end:   end,
		used:  make(map[int]struct{}),
	}
}
