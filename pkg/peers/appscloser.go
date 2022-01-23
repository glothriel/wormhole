package peers

// autoClosingAppsChanPeerFactory is a decorator over PeerFactory, that automatically
// closes apps channel when peer is terminated
type autoClosingAppsChanPeerFactory struct {
	child PeerFactory
}

// Peers implements PeerFactory
func (uniq *autoClosingAppsChanPeerFactory) Peers() (chan Peer, error) {
	childResult, childErr := uniq.child.Peers()
	if childErr != nil {
		return childResult, childErr
	}
	proxyChannel := make(chan Peer)
	go func() {
		defer close(proxyChannel)
		for peer := range childResult {
			peer.WhenClosed(func() {
				close(peer.AppStatusChanges())
			})
			proxyChannel <- peer
		}
	}()
	return proxyChannel, nil
}

// AutoCloseAppsChan creates autoClosingAppsChanPeerFactory instances
func AutoCloseAppsChan(child PeerFactory) PeerFactory {
	return &autoClosingAppsChanPeerFactory{
		child: child,
	}
}
