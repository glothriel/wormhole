package peers

import (
	"sync"

	"github.com/sirupsen/logrus"
)

// uniquePeerFactory is a decorator over PeerFactory, that allows connecting only peers with unique names
type uniquePeerFactory struct {
	child PeerFactory
}

// Peers implements PeerFactory
func (uniq *uniquePeerFactory) Peers() (chan Peer, error) {
	childResult, childErr := uniq.child.Peers()
	if childErr != nil {
		return childResult, childErr
	}
	proxyChannel := make(chan Peer)
	go func() {
		defer close(proxyChannel)
		alreadyConnectedPeers := sync.Map{}
		for peer := range childResult {
			_, peerAlreadyConnected := alreadyConnectedPeers.Load(peer.Name())
			if peerAlreadyConnected {
				logrus.Warnf("Peer with name `%s` is already connected, denying new one", peer.Name())
				if closeErr := peer.Close(); closeErr != nil {
					logrus.Errorf("Unable to close peer: %v", closeErr)
				}
				continue
			}
			alreadyConnectedPeers.Store(peer.Name(), true)
			peer.WhenClosed(func() {
				alreadyConnectedPeers.Delete(peer.Name())
			})
			proxyChannel <- peer
		}
	}()
	return proxyChannel, nil
}

// AllowOnlyUniquePeers creates uniquePeerFactory instances
func AllowOnlyUniquePeers(child PeerFactory) PeerFactory {
	return &uniquePeerFactory{
		child: child,
	}
}
