package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glothriel/wormhole/pkg/pairing"
	"github.com/glothriel/wormhole/pkg/syncing"
	"github.com/glothriel/wormhole/pkg/wg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingPeerDeleter struct {
	deleted []string
}

func (d *recordingPeerDeleter) DeletePeer(peer string, deleteRecord func() error) error {
	d.deleted = append(d.deleted, peer)
	return deleteRecord()
}

func setupPeersAPI(t *testing.T) (*gin.Engine, pairing.PeerStorage, *recordingPeerDeleter) {
	gin.SetMode(gin.TestMode)
	peers := pairing.NewInMemoryPeerStorage()
	require.NoError(t, peers.Store(pairing.PeerInfo{Name: "peer1", IP: "10.0.0.2", PublicKey: "pub1"}))
	wgConfig := &wg.Config{Peers: []wg.Peer{{Name: "peer1", PublicKey: "pub1", AllowedIPs: "10.0.0.2/32"}}}
	deleter := &recordingPeerDeleter{}
	controller := NewPeersController(
		peers,
		wgConfig,
		wg.NewWatcher(filepath.Join(t.TempDir(), "wg0.conf")),
		syncing.NewInMemoryMetadataStorage(),
		deleter,
	)
	r := gin.New()
	controller.registerRoutes(r, ServerSettings{
		BasicAuthEnabled:  true,
		BasicAuthUsername: "user",
		BasicAuthPassword: "pass",
	})
	return r, peers, deleter
}

func deletePeerRequest(r *gin.Engine, name string, withAuth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/api/peers/v1/"+name, nil)
	if withAuth {
		req.SetBasicAuth("user", "pass")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestDeletePeerWithoutCredentialsIsRejected(t *testing.T) {
	r, peers, deleter := setupPeersAPI(t)

	w := deletePeerRequest(r, "peer1", false)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, deleter.deleted)
	_, err := peers.GetByName("peer1")
	assert.NoError(t, err)
}

func TestDeleteUnknownPeerReturnsError(t *testing.T) {
	r, _, _ := setupPeersAPI(t)

	w := deletePeerRequest(r, "unknown", true)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDeleteKnownPeerGoesThroughDeleter(t *testing.T) {
	r, peers, deleter := setupPeersAPI(t)

	w := deletePeerRequest(r, "peer1", true)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, []string{"peer1"}, deleter.deleted)
	_, err := peers.GetByName("peer1")
	assert.ErrorIs(t, err, pairing.ErrPeerDoesNotExist)
}
