package router

import (
	"sync"

	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/sirupsen/logrus"
)

// PacketRouter implements server.messageRouter
// It routes messages comming from peers to per-session sub-channels, so they cay be later piped
// directly to TCP connections
type PacketRouter struct {
	perSessionMailboxes map[string]chan messages.Message

	lock *sync.Mutex
}

func (router *PacketRouter) put(msg messages.Message) {
	router.ensureMailbox(msg.SessionID) <- msg
}

// Get allows retrieving a channel for specific session ID
func (router *PacketRouter) Get(sessionID string) chan messages.Message {
	return router.ensureMailbox(sessionID)
}

// Done can be sued to mark the channel for sessionID for deletion
func (router *PacketRouter) Done(sessionID string) {
	router.locked(func() {
		_, mailboxExists := router.perSessionMailboxes[sessionID]
		if mailboxExists {
			close(router.perSessionMailboxes[sessionID])
		}
		delete(router.perSessionMailboxes, sessionID)
	})
}

func (router *PacketRouter) ensureMailbox(sessionID string) chan messages.Message {
	var mailbox chan messages.Message
	router.locked(func() {
		_, mailboxExists := router.perSessionMailboxes[sessionID]
		if !mailboxExists {
			router.perSessionMailboxes[sessionID] = make(chan messages.Message)
		}
		mailbox = router.perSessionMailboxes[sessionID]
	})
	return mailbox
}

func (router *PacketRouter) locked(f func()) {
	router.lock.Lock()
	defer router.lock.Unlock()
	f()
}

// NewPacketRouter creates MessageRouter instances
func NewPacketRouter(allMessages chan messages.Message) *PacketRouter {
	theRouter := &PacketRouter{
		lock:                &sync.Mutex{},
		perSessionMailboxes: make(map[string]chan messages.Message),
	}
	go func(router *PacketRouter, msgs chan messages.Message) {
		for message := range msgs {
			if messages.IsPacket(message) {
				router.put(message)
			} else {
				logrus.Errorf("Unexpected message type %s passed to PacketRouter", message.Type)
			}
		}
		// Once the upstream channel closes, remove all remaining sessions
		for sessionID := range router.perSessionMailboxes {
			router.Done(sessionID)
		}
	}(theRouter, allMessages)
	return theRouter
}
