package router

import (
	"sync"

	"github.com/glothriel/wormhole/pkg/grtn"
	"github.com/glothriel/wormhole/pkg/messages"
)

// MessageRouter implements server.messageRouter
// It routes messages comming from peers to per-session sub-channels, so they cay be later piped
// directly to TCP connections
type MessageRouter struct {
	perSessionMailboxes map[string]chan messages.Message

	lock *sync.Mutex
}

func (router *MessageRouter) put(msg messages.Message) {
	router.ensureMailbox(msg.SessionID) <- msg
}

// Get allows retrieving a channel for specific session ID
func (router *MessageRouter) Get(sessionID string) chan messages.Message {
	return router.ensureMailbox(sessionID)
}

// Done can be sued to mark the channel for sessionID for deletion
func (router *MessageRouter) Done(sessionID string) {
	router.locked(func() {
		_, mailboxExists := router.perSessionMailboxes[sessionID]
		if mailboxExists {
			close(router.perSessionMailboxes[sessionID])
		}
		delete(router.perSessionMailboxes, sessionID)
	})
}

func (router *MessageRouter) ensureMailbox(sessionID string) chan messages.Message {
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

func (router *MessageRouter) locked(f func()) {
	router.lock.Lock()
	defer router.lock.Unlock()
	f()
}

// NewMessageRouter creates MessageRouter instances
func NewMessageRouter(allMessages chan messages.Message) *MessageRouter {
	theRouter := &MessageRouter{
		lock:                &sync.Mutex{},
		perSessionMailboxes: make(map[string]chan messages.Message),
	}
	grtn.GoA2[*MessageRouter, chan messages.Message](func(router *MessageRouter, msgs chan messages.Message) {
		for message := range msgs {
			if messages.IsFrame(message) {
				router.put(message)
			}
		}
		// Once the upstream channel closes, remove all remaining sessions
		for sessionID := range router.perSessionMailboxes {
			router.Done(sessionID)
		}
	}, theRouter, allMessages)
	return theRouter
}
