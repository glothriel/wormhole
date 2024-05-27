package peers

import "sync"

// Peer represents a peer in the network (either a client or a server)
type Peer string

// Registry is an interface for managing peers
type Registry interface {
	Add(Peer)
	Exists(Peer) bool
	Remove(Peer)
	List() []Peer
}

type registry struct {
	data sync.Map
}

func (r *registry) Add(peer Peer) {
	r.data.Store(peer, struct{}{})
}

func (r *registry) Exists(peer Peer) bool {
	_, ok := r.data.Load(peer)
	return ok
}

func (r *registry) Remove(peer Peer) {
	r.data.Delete(peer)
}

func (r *registry) List() []Peer {
	peers := make([]Peer, 0)
	r.data.Range(func(key, _ any) bool {
		peers = append(peers, key.(Peer))
		return true
	})
	return peers
}

// NewRegistry creates a new registry of peers
func NewRegistry() Registry {
	return &registry{}
}
