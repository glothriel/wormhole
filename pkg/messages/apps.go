package messages

import "strings"

// AppEventsEncode encodes appName and address into message
func AppEventsEncode(appName, address string) string {
	return strings.Join([]string{appName, address}, ",")
}

// AppEventsDecode decodes message body and allows exracting appName and address
func AppEventsDecode(body string) (string, string) {
	elems := strings.Split(body, ",")
	return elems[0], elems[1]
}
