package nginx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockLister struct {
	interfaces []networkInterface
	err        error
}

func (m *mockLister) Interfaces() ([]networkInterface, error) {
	return m.interfaces, m.err
}

func TestOnlyWgListener(t *testing.T) {
	// given
	lister := &mockLister{
		interfaces: []networkInterface{
			{
				name:      "eth0",
				addresses: []string{"127.0.0.1"},
			},
			{
				name:      "wg0",
				addresses: []string{"10.178.2.1"},
			},
		},
	}
	listenerIf := NewOnlyWireguardListener()
	listener := listenerIf.(*wg0FilteringListener)
	listener.lister = lister

	// when
	addrs, err := listener.Addrs(80)

	// then
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"10.178.2.1:80"}, addrs)
}

func TestAllAcceptWgListener(t *testing.T) {
	// given
	lister := &mockLister{
		interfaces: []networkInterface{
			{
				name:      "eth0",
				addresses: []string{"127.0.0.1"},
			},
			{
				name:      "wg0",
				addresses: []string{"10.178.2.1"},
			},
		},
	}
	listenerIf := NewAllAcceptWireguardListener()
	listener := listenerIf.(*wg0FilteringListener)
	listener.lister = lister

	// when
	addrs, err := listener.Addrs(80)

	// then
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"127.0.0.1:80"}, addrs)
}
