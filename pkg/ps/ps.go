package ps

import (
	"regexp"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Context struct {
	Data sync.Map
}

func CtxGet[T any](ctx *Context, key string) (T, bool) {
	v, ok := ctx.Data.Load(key)
	var t T
	if !ok {
		return t, false
	}
	rv, ok := v.(T)
	if !ok {
		return t, false
	}
	return rv, true
}

func NewContext() *Context {
	return &Context{
		Data: sync.Map{},
	}
}

func CtxSet(ctx *Context, key string, value any) {
	ctx.Data.Store(key, value)
}

type HandlerFunc func(*Context, any) error

type Handler struct {
	ClientID string
	Handler  HandlerFunc
}

type PubSub interface {
	Subscribe(pattern string, handler func(*Context, any) error, clientID ...string)
	Publish(topic string, ctx *Context, message any) error
	Unsubscribe(clientID string)
}

type InMemoryPubSub struct {
	subscribers map[*regexp.Regexp][]Handler
}

func (i *InMemoryPubSub) Subscribe(pattern string, handler func(*Context, any) error, ClientID ...string) {
	theClientID := "default"
	if len(ClientID) > 0 {
		theClientID = ClientID[0]
	}
	logrus.Debugf("[BUS] Subscriber registered to %s", pattern)
	r, err := regexp.Compile(pattern)
	if err != nil {
		// Handle error
		return
	}
	i.subscribers[r] = append(i.subscribers[r], Handler{
		ClientID: theClientID,
		Handler:  handler,
	})
}

func (i *InMemoryPubSub) Publish(topic string, ctx *Context, v any) error {

	startedAt := time.Now()
	numHandled := 0
	for r, handlers := range i.subscribers {
		if r.MatchString(topic) {
			for _, handler := range handlers {
				numHandled++
				if err := handler.Handler(ctx, v); err != nil {
					return err
				}
			}
		}
	}
	logrus.Debugf("[BUS] Published %s to %d handlers in %dus", topic, numHandled, time.Since(startedAt).Microseconds())
	return nil
}

func (i *InMemoryPubSub) Unsubscribe(clientID string) {
	for r, handlers := range i.subscribers {
		for n, handler := range handlers {
			if handler.ClientID == clientID {
				i.subscribers[r] = append(handlers[:n], handlers[n+1:]...)
				logrus.Debugf("[BUS] Unsubscribed %s from %s", clientID, r.String())
			}
		}
	}
}

func NewInMemoryPubSub() PubSub {
	return &InMemoryPubSub{
		subscribers: make(map[*regexp.Regexp][]Handler),
	}
}
