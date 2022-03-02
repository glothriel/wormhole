package messages

import "strings"

// AppStatusChanged is event emmited when application status is changed
type AppStatusChanged struct {
	App    string
	Status string
}

// AppAddedEncode encodes appName and address into message
func AppAddedEncode(appName, address string) string {
	return strings.Join([]string{appName, address}, ",")
}

// AppAddedDecode decodes message body and allows exracting appName and address
func AppAddedDecode(body string) (string, string) {
	elems := strings.Split(body, ",")
	return elems[0], elems[1]
}
