package hello

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type httpServerTransport struct {
	requests chan IncomingPairingRequest
	server   *http.Server
}

func (t *httpServerTransport) Requests() <-chan IncomingPairingRequest {
	return t.requests
}

func NewHTTPServerTransport(server *http.Server) PairingServerTransport {
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
			logrus.Fatalf("Failed to start HTTP transport server: %v", err)
		}
	}()
	return &httpServerTransport{
		requests: incoming,
		server:   server,
	}
}

type httpClientTransport struct {
	serverURL string
	client    *http.Client
}

func (t *httpClientTransport) Send(req []byte) ([]byte, error) {
	postURL := t.serverURL + "/pairing"
	resp, err := t.client.Post(postURL, "application/octet-stream", bytes.NewReader(req))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Server returned status code %d when called %s", resp.StatusCode, postURL)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

func NewHTTPClientTransport(serverURL string) PairingClientTransport {
	return &httpClientTransport{
		serverURL: serverURL,
		client:    &http.Client{},
	}
}
