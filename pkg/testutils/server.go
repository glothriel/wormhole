package testutils

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// RunTestServer starts test server, that is used as example app for integration tests
func RunTestServer(port int, response string) error {
	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(200)
		if _, writeErr := rw.Write([]byte(response)); writeErr != nil {
			logrus.Errorf("Failed to write message: %s", writeErr)
		}
	})
	server := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", port),
		ReadHeaderTimeout: 3 * time.Second,
	}
	return server.ListenAndServe()
}
