package admin

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"sync"

	"github.com/glothriel/wormhole/pkg/auth"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// ServerAcceptor implements Acceptor by waiting for the user to manually accept the public key
type ServerAcceptor struct {
	gatherer *ConsentGatherer
}

// IsTrusted implements Acceptor
func (a *ServerAcceptor) IsTrusted(cert *rsa.PublicKey) (bool, error) {
	logrus.Infof(
		"New peer connected, awaiting fingerprint approval: %s", auth.Fingerprint(cert),
	)
	result, askErr := a.gatherer.Ask(cert)
	if askErr != nil {
		return false, askErr
	}
	return <-result, nil
}

// NewServerAcceptor creates ServerAcceptor instances
func NewServerAcceptor(gatherer *ConsentGatherer) *ServerAcceptor {
	return &ServerAcceptor{
		gatherer: gatherer,
	}
}

// ConsentGatherer is a mediator between ServerAcceptor and HTTP server that is used to accept or reject the requests
type ConsentGatherer struct {
	requests sync.Map
	maxCount int
}

// Ask returns a channel, that determines if consent was given or not, after the user responds
func (gatherer *ConsentGatherer) Ask(cert *rsa.PublicKey) (chan bool, error) {
	totalCount := 0
	// It's racy, but it doesn't really matter here
	gatherer.requests.Range(func(key, value any) bool {
		totalCount++
		return true
	})
	if totalCount > gatherer.maxCount {
		return nil, errors.New("Too much simultaneous requests")
	}
	rawChan, found := gatherer.requests.Load(auth.Fingerprint(cert))
	if found {
		logrus.Infof("Found previous consent request for fingerprint %s - closing it", auth.Fingerprint(cert))
		theChan := rawChan.(chan bool)
		theChan <- false
		close(theChan)
		gatherer.requests.Delete(auth.Fingerprint(cert))
	}
	theChan := make(chan bool)
	gatherer.requests.Store(auth.Fingerprint(cert), theChan)
	return theChan, nil
}

// NewConsentGatherer creates ConsentGatherer instances
func NewConsentGatherer() *ConsentGatherer {
	return &ConsentGatherer{
		maxCount: 32,
		requests: sync.Map{},
	}
}

func listAcceptRequests(gatherer *ConsentGatherer) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var pendingFingerprints []string
		gatherer.requests.Range(func(key, value any) bool {
			fingerprint, ok := key.(string)
			if !ok {
				return true
			}
			pendingFingerprints = append(pendingFingerprints, fingerprint)
			return true
		})

		pendingFingerprintsEncodedToJSON, marshalErr := json.Marshal(pendingFingerprints)
		if marshalErr != nil {
			rw.WriteHeader(500)
			logrus.Error(marshalErr)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(200)
		if _, writeErr := rw.Write(pendingFingerprintsEncodedToJSON); writeErr != nil {
			logrus.Error(writeErr)
		}
	}
}

func updateAcceptRequest(gatherer *ConsentGatherer) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		fingerprint := mux.Vars(r)["fingerprint"]
		rawChannel, ok := gatherer.requests.Load(fingerprint)
		if !ok {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		theChannel, ok := rawChannel.(chan bool)
		if !ok {
			logrus.Errorf("Invalid type of item kept in ConsentGatherer: %s", reflect.TypeOf(theChannel).Name())
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer func() {
			gatherer.requests.Delete(fingerprint)
			close(theChannel)
		}()
		if r.Method == "POST" {
			theChannel <- true
		} else {
			theChannel <- false
		}
		rw.WriteHeader(http.StatusNoContent)
	}
}
