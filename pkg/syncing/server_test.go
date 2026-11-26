package syncing

import (
	"errors"
	"sync"
	"testing"

	"github.com/glothriel/wormhole/pkg/apps"
	"github.com/glothriel/wormhole/pkg/listeners"
	"github.com/glothriel/wormhole/pkg/pairing"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockServerTransport struct {
	syncs chan IncomingSyncRequest
}

func (t *mockServerTransport) Syncs() <-chan IncomingSyncRequest {
	return t.syncs
}

func (t *mockServerTransport) Metadata() map[string]string {
	return map[string]string{}
}

// failingExposer fails Withdraw for the app named failWithdraw and records every withdrawn app
type failingExposer struct {
	failWithdraw string
	withdrawn    []apps.App
}

func (e *failingExposer) Add(app apps.App) (apps.App, error) {
	return app, nil
}

func (e *failingExposer) Withdraw(app apps.App) error {
	e.withdrawn = append(e.withdrawn, app)
	if app.Name == e.failWithdraw {
		return errors.New("withdraw failed")
	}
	return nil
}

func (e *failingExposer) WithdrawAll() error {
	return nil
}

type failingDeleteMetadataStorage struct {
	MetadataStorage
}

func (s *failingDeleteMetadataStorage) Delete(_ string) error {
	return errors.New("metadata delete failed")
}

type serverFixture struct {
	server    *Server
	transport *mockServerTransport
	peers     pairing.PeerStorage
	metadata  MetadataStorage
	exposer   *failingExposer
	registry  *listeners.Registry
	done      chan bool
}

func newServerFixture(t *testing.T, metadata MetadataStorage, exposer *failingExposer) *serverFixture {
	generator := NewAppStateChangeGenerator()
	f := &serverFixture{
		transport: &mockServerTransport{syncs: make(chan IncomingSyncRequest)},
		peers:     pairing.NewInMemoryPeerStorage(),
		metadata:  metadata,
		exposer:   exposer,
		registry:  listeners.NewApps(exposer),
		done:      make(chan bool),
	}
	f.server = NewServer(
		"server", generator, NewInMemoryAppStorage(), NewJSONSyncingEncoder(), f.transport, f.peers, metadata,
	)
	require.NoError(t, f.peers.Store(pairing.PeerInfo{Name: "p1"}))
	require.NoError(t, f.peers.Store(pairing.PeerInfo{Name: "p2"}))
	go f.registry.Watch(generator.Changes(), f.done)
	go f.server.Start()
	t.Cleanup(func() { close(f.transport.syncs) })
	return f
}

func (f *serverFixture) sync(peer string, theApps []apps.App) error {
	encoded, encodeErr := NewJSONSyncingEncoder().Encode(Message{
		Peer:     peer,
		Metadata: Metadata{"peer": peer},
		Apps:     theApps,
	})
	if encodeErr != nil {
		return encodeErr
	}
	req := IncomingSyncRequest{Request: encoded, Response: make(chan []byte), Err: make(chan error)}
	f.transport.syncs <- req
	select {
	case <-req.Response:
		return nil
	case err := <-req.Err:
		return err
	}
}

func (f *serverFixture) deletePeer(peer string) error {
	return f.server.DeletePeer(peer, func() error { return f.peers.DeleteByName(peer) })
}

// stopWatch stops the registry's Watch loop, after which every emitted change has been processed
func (f *serverFixture) stopWatch() {
	f.done <- true
}

func (f *serverFixture) appsOf(t *testing.T, peer string) []string {
	theApps, err := f.registry.List()
	require.NoError(t, err)
	names := make([]string, 0)
	for _, app := range theApps {
		if app.Peer == peer {
			names = append(names, app.Name)
		}
	}
	return names
}

func TestServerDeletePeerRemovesOnlyItsAppsAndMetadata(t *testing.T) {
	f := newServerFixture(t, NewInMemoryMetadataStorage(), &failingExposer{})
	require.NoError(t, f.sync("p1", []apps.App{{Name: "x", Address: "p1:1"}, {Name: "y", Address: "p1:2"}}))
	require.NoError(t, f.sync("p2", []apps.App{{Name: "x", Address: "p2:1"}}))

	require.NoError(t, f.deletePeer("p1"))

	_, getErr := f.metadata.Get("p1")
	assert.ErrorIs(t, getErr, ErrPeerNotFound)
	p2Metadata, p2Err := f.metadata.Get("p2")
	require.NoError(t, p2Err)
	assert.Equal(t, Metadata{"peer": "p2"}, p2Metadata)
	f.stopWatch()
	assert.Empty(t, f.appsOf(t, "p1"))
	assert.Equal(t, []string{"x"}, f.appsOf(t, "p2"))
}

func TestServerRejectsSyncFromDeletedPeer(t *testing.T) {
	f := newServerFixture(t, NewInMemoryMetadataStorage(), &failingExposer{})
	require.NoError(t, f.sync("p1", []apps.App{{Name: "x", Address: "p1:1"}}))
	require.NoError(t, f.deletePeer("p1"))

	syncErr := f.sync("p1", []apps.App{{Name: "x", Address: "p1:1"}})

	assert.ErrorIs(t, syncErr, pairing.ErrPeerDoesNotExist)
	_, getErr := f.metadata.Get("p1")
	assert.ErrorIs(t, getErr, ErrPeerNotFound)
	f.stopWatch()
	assert.Empty(t, f.appsOf(t, "p1"))
}

func TestServerDeletePeerRecordFailureCleansUpNothing(t *testing.T) {
	f := newServerFixture(t, NewInMemoryMetadataStorage(), &failingExposer{})
	require.NoError(t, f.sync("p1", []apps.App{{Name: "x", Address: "p1:1"}}))
	recordErr := errors.New("record delete failed")

	deleteErr := f.server.DeletePeer("p1", func() error { return recordErr })

	assert.ErrorIs(t, deleteErr, recordErr)
	_, getErr := f.metadata.Get("p1")
	assert.NoError(t, getErr)
	f.stopWatch()
	assert.Equal(t, []string{"x"}, f.appsOf(t, "p1"))
	assert.Empty(t, f.exposer.withdrawn)
}

func TestServerDeletePeerWithdrawsAllAppsWhenOneWithdrawFails(t *testing.T) {
	hook := logrustest.NewGlobal()
	defer hook.Reset()
	f := newServerFixture(t, NewInMemoryMetadataStorage(), &failingExposer{failWithdraw: "x"})
	require.NoError(t, f.sync("p1", []apps.App{{Name: "x", Address: "p1:1"}, {Name: "y", Address: "p1:2"}}))

	require.NoError(t, f.deletePeer("p1"))

	f.stopWatch()
	assert.Empty(t, f.appsOf(t, "p1"))
	assert.Len(t, f.exposer.withdrawn, 2)
	assert.True(t, hasErrorLog(hook, "Could not withdraw app: withdraw failed"))
}

func TestServerDeletePeerWithdrawsAppsWhenMetadataDeleteFails(t *testing.T) {
	hook := logrustest.NewGlobal()
	defer hook.Reset()
	f := newServerFixture(
		t, &failingDeleteMetadataStorage{NewInMemoryMetadataStorage()}, &failingExposer{},
	)
	require.NoError(t, f.sync("p1", []apps.App{{Name: "x", Address: "p1:1"}, {Name: "y", Address: "p1:2"}}))

	require.NoError(t, f.deletePeer("p1"))

	f.stopWatch()
	assert.Empty(t, f.appsOf(t, "p1"))
	assert.True(t, hasErrorLog(hook, "Could not delete metadata of peer p1: metadata delete failed"))
}

func TestServerConcurrentSyncAndDeletePeerLeavesNothingBehind(t *testing.T) {
	for i := 0; i < 50; i++ {
		f := newServerFixture(t, NewInMemoryMetadataStorage(), &failingExposer{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = f.sync("p1", []apps.App{{Name: "x", Address: "p1:1"}})
		}()
		go func() {
			defer wg.Done()
			assert.NoError(t, f.deletePeer("p1"))
		}()
		wg.Wait()

		_, getErr := f.metadata.Get("p1")
		assert.ErrorIs(t, getErr, ErrPeerNotFound)
		f.stopWatch()
		assert.Empty(t, f.appsOf(t, "p1"))
	}
}

func hasErrorLog(hook *logrustest.Hook, message string) bool {
	for _, entry := range hook.AllEntries() {
		if entry.Level == logrus.ErrorLevel && entry.Message == message {
			return true
		}
	}
	return false
}
