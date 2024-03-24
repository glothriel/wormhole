package peers

import "sync"

type Peer string

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
	r.data.Range(func(key, value interface{}) bool {
		peers = append(peers, key.(Peer))
		return true
	})
	return peers
}

func NewRegistry() Registry {
	return &registry{}
}
