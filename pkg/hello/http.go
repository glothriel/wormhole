package hello

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type httpServerPairingTransport struct {
	requests chan IncomingPairingRequest
	server   *http.Server
}

func (t *httpServerPairingTransport) Requests() <-chan IncomingPairingRequest {
	return t.requests
}

func NewHTTPServerPairingTransport(server *http.Server) PairingServerTransport {
	incoming := make(chan IncomingPairingRequest)
	router := mux.NewRouter()
	router.HandleFunc("/pairing", func(w http.ResponseWriter, r *http.Request) {
		var req IncomingPairingRequest
		req.Request = make([]byte, r.ContentLength)
		r.Body.Read(req.Request)
		req.Response = make(chan []byte)
		req.Err = make(chan error)
		incoming <- req
		select {
		case resp := <-req.Response:
			w.Write(resp)
		case err := <-req.Err:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	server.Handler = router
	go func() {
		logrus.Infof("Starting HTTP pairing transport server on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil {
			logrus.Fatalf("Failed to start HTTP pairing transport server: %v", err)
		}
	}()
	return &httpServerPairingTransport{
		requests: incoming,
		server:   server,
	}
}

type httpClientPairingTransport struct {
	serverURL string
	client    *http.Client
}

func (t *httpClientPairingTransport) Send(req []byte) ([]byte, error) {
	postURL := t.serverURL + "/pairing"
	resp, err := t.client.Post(postURL, "application/octet-stream", bytes.NewReader(req))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody := make([]byte, resp.ContentLength)
		_, readErr := resp.Body.Read(respBody)
		if readErr != nil {
			logrus.Errorf("Failed to read response body: %v", readErr)
		}
		return nil, fmt.Errorf("Server returned status code %d when called %s: %s", resp.StatusCode, postURL, string(respBody))
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

func NewHTTPClientPairingTransport(serverURL string) PairingClientTransport {
	return &httpClientPairingTransport{
		serverURL: serverURL,
		client:    &http.Client{},
	}
}

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

func NewHTTPServerSyncingTransport(server *http.Server) SyncServerTransport {
	syncs := make(chan IncomingSyncRequest)
	router := http.NewServeMux()
	router.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		var req IncomingSyncRequest
		req.Request = make([]byte, r.ContentLength)
		r.Body.Read(req.Request)
		req.Response = make(chan []byte)
		req.Err = make(chan error)
		syncs <- req
		select {
		case resp := <-req.Response:
			w.Write(resp)
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
	resp.Body.Read(respBody)
	return respBody, nil
}

func NewHTTPClientSyncingTransport(serverURL string) SyncClientTransport {
	return &httpClientSyncingTransport{
		serverURL: serverURL,
		client:    &http.Client{},
	}
}
