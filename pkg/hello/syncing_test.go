package hello

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockSyncClientTransport struct {
	lastCalledWith []byte
	returnValue    []byte
	returnError    error
}

func (m *mockSyncClientTransport) Sync(data []byte) ([]byte, error) {
	m.lastCalledWith = data
	return m.returnValue, m.returnError
}

func TestClientStartFailsAfterXSyncFailures(t *testing.T) {
	// given
	client := NewSyncingClient(
		"client",
		NewAppStateChangeGenerator(),
		NewJSONSyncingEncoder(),
		1,
		NewInMemoryAppStorage(),
		&mockSyncClientTransport{
			returnError: fmt.Errorf("sync failed"),
		},
	)

	// when
	startErr := client.Start()

	// then
	assert.Error(t, startErr)
	assert.Contains(t, startErr.Error(), "failed to sync 3 times in a row")
}
