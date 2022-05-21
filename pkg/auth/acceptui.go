package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// ListPairingRequests displays a list of pairing requests
func ListPairingRequests(serverURL string) ([]string, error) {
	var requests []string
	theURL, parseErr := url.Parse(serverURL)
	if parseErr != nil {
		return requests, fmt.Errorf("Could not parse server URL: %w", parseErr)
	}
	theURL.Path = "/v1/requests"
	response, responseErr := http.Get(theURL.String())
	if responseErr != nil {
		return requests, fmt.Errorf("Error when contacting wormhole API: %w", responseErr)
	}
	if response.StatusCode != http.StatusOK {
		return requests, fmt.Errorf("Unexpected status when contacting wormhole API: %s", response.Status)
	}
	bodyBytes, readAllErr := io.ReadAll(response.Body)
	if readAllErr != nil {
		return requests, fmt.Errorf("Error when reading wormhole API response: %w", readAllErr)
	}
	unmarshalErr := json.Unmarshal(bodyBytes, &requests)
	if unmarshalErr != nil {
		return requests, fmt.Errorf("Error when decoding JSON from wormhole API: %w", unmarshalErr)
	}
	return requests, nil
}

// AcceptRequest accepts pairing request fingerprint
func AcceptRequest(serverURL string, fingerprint string) error {
	return doRequestOnFingerprintDetails(serverURL, fingerprint, "POST")
}

// DeclineRequest accepts pairing request fingerprint
func DeclineRequest(serverURL string, fingerprint string) error {
	return doRequestOnFingerprintDetails(serverURL, fingerprint, "DELETE")
}

func doRequestOnFingerprintDetails(serverURL string, fingerprint string, method string) error {
	theURL, parseErr := url.Parse(serverURL)
	if parseErr != nil {
		return fmt.Errorf("Could not parse server URL: %w", parseErr)
	}
	theURL.Path = fmt.Sprintf("/v1/requests/%s", fingerprint)

	request, newRequestErr := http.NewRequest(method, theURL.String(), nil)
	if newRequestErr != nil {
		return fmt.Errorf("Error when constructing request: %w", newRequestErr)
	}
	response, responseErr := (&http.Client{}).Do(request)
	if responseErr != nil {
		return fmt.Errorf("Error when contacting wormhole API: %w", responseErr)
	}
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Unexpected status when contacting wormhole API: %s", response.Status)
	}
	return nil
}
