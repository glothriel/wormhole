package testutils

import (
	"fmt"
	"net/http"
)

// RunTestServer starts test server, that is used as example app for integration tests
func RunTestServer(port int, response string) error {
	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(200)
		rw.Write([]byte(response))
	})
	return http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), nil)
}
