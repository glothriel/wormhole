package nginx

import (
	"errors"
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
	listener := listenerIf.(*allAcceptWg0Listener)
	listener.lister = lister

	// when
	addrs, err := listener.Addrs(80)

	// then
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"127.0.0.1:80"}, addrs)
}
func TestAllAcceptWgListenerErrors(t *testing.T) {
	tests := []struct {
		name       string
		interfaces []networkInterface
		err        error
		expected   []string
	}{
		{
			name: "Nonempty interface list with error",
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
			err:      errors.New("Blabla"),
			expected: nil,
		},
		{
			name:       "Empty interface list without error",
			interfaces: []networkInterface{},
			err:        nil,
			expected:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister := &mockLister{
				interfaces: tt.interfaces,
				err:        tt.err,
			}
			listenerIf := NewAllAcceptWireguardListener()
			listener := listenerIf.(*allAcceptWg0Listener)
			listener.lister = lister

			_, err := listener.Addrs(80)

			assert.Error(t, err)
		})
	}
}

func TestGivenAddressOnlyListener(t *testing.T) {
	// given
	listener := NewOnlyGivenAddressListener("127.0.0.1")

	// when
	addrs, err := listener.Addrs(80)

	// then
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"127.0.0.1:80"}, addrs)
}
