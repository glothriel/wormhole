package router

import (
	"fmt"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/glothriel/wormhole/pkg/messages"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestMessagesAreProperlyRouted(t *testing.T) {
	messagesChannel := make(chan messages.Message)
	theRouter := NewMessageRouter(messagesChannel)

	sessionIDs := []string{}
	for i := 0; i < 10000; i++ {
		sessionIDs = append(sessionIDs, uuid.New().String())
	}
	for _, sessionID := range sessionIDs {
		go func(theSessID string) { theRouter.Get(theSessID) <- messages.Message{SessionID: theSessID} }(sessionID)
	}

	for _, sessionID := range sessionIDs {
		assert.Equal(t, sessionID, (<-theRouter.Get(sessionID)).SessionID)
	}
	close(messagesChannel)
}

func TestMailboxesAreClosedOnceMasterChannelIsClosed(t *testing.T) {
	messagesChannel := make(chan messages.Message)
	theRouter := NewMessageRouter(messagesChannel)

	go func() { theRouter.Get("bla") <- messages.Message{SessionID: "bla"} }()
	<-theRouter.Get("bla")
	close(messagesChannel)

	assert.Nil(t, retry.Do(
		func() error {
			if len(theRouter.perSessionMailboxes) != 0 {
				return fmt.Errorf("There should be 0 mailboxes, found %d", len(theRouter.perSessionMailboxes))
			}
			return nil
		},
		retry.Attempts(10),
		retry.Delay(time.Millisecond*100),
	))
}
