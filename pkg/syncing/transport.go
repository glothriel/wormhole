package syncing

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type httpServerSyncingTransport struct {
	syncs  chan IncomingSyncRequest
	server *http.Server
}

func (t *httpServerSyncingTransport) Syncs() <-chan IncomingSyncRequest {
	return t.syncs
}

func (t *httpServerSyncingTransport) Metadata() map[string]string {
	return map[string]string{
		"sync_server_address": fmt.Sprintf("http://%s", t.server.Addr),
	}
}

// NewHTTPServerSyncingTransport creates a new SyncServerTransport instance
func NewHTTPServerSyncingTransport(server *http.Server) SyncServerTransport {
	syncs := make(chan IncomingSyncRequest)
	router := http.NewServeMux()
	router.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) { // nolint: dupl
		var req IncomingSyncRequest
		req.Request = make([]byte, r.ContentLength)
		_, readErr := r.Body.Read(req.Request)
		if readErr != nil && readErr != io.EOF {
			http.Error(w, readErr.Error(), http.StatusInternalServerError)
			return
		}
		req.Response = make(chan []byte)
		req.Err = make(chan error)
		syncs <- req
		select {
		case resp := <-req.Response:
			_, writeErr := w.Write(resp)
			if writeErr != nil {
				http.Error(w, writeErr.Error(), http.StatusInternalServerError)
			}
		case err := <-req.Err:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	server.Handler = router

	go func() {
		for {
			logrus.Infof("Starting HTTP syncing transport server on %s", server.Addr)
			if err := server.ListenAndServe(); err != nil {
				logrus.Errorf("Failed to start HTTP syncing transport server: %v", err)
				time.Sleep(time.Second * 5)
			}
		}
	}()
	return &httpServerSyncingTransport{
		syncs:  syncs,
		server: server,
	}
}

type httpClientSyncingTransport struct {
	serverURL string
	client    *http.Client
}

func (t *httpClientSyncingTransport) Sync(req []byte) ([]byte, error) {
	resp, err := t.client.Post(t.serverURL+"/sync", "application/octet-stream", bytes.NewReader(req))
	if err != nil {
		return nil, err
	}
	respBody := make([]byte, resp.ContentLength)
	_, readErr := resp.Body.Read(respBody)
	if readErr != nil && readErr != io.EOF {
		return nil, readErr
	}
	return respBody, nil
}

// NewHTTPClientSyncingTransport creates a new SyncClientTransport instance
func NewHTTPClientSyncingTransport(serverURL string, timeout time.Duration) SyncClientTransport {
	return &httpClientSyncingTransport{
		serverURL: serverURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}
