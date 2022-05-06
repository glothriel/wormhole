package messages

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// SerializeBytes serializes the message for transit over the wire
func SerializeBytes(m Message) []byte {
	b, marshalErr := json.Marshal(WithBody(m, base64.StdEncoding.EncodeToString([]byte(m.BodyString))))
	if marshalErr != nil {
		panic(marshalErr)
	}
	return b
}

// Serialize serializes the message for debugging or for text-based wire protocols
func Serialize(m Message) string {
	return string(SerializeBytes(m))
}

// DeserializeMessageBytes Deserializes message from bytes
func DeserializeMessageBytes(b []byte) (Message, error) {
	theMsg := Message{}
	unmarshalErr := json.Unmarshal(b, &theMsg)
	if unmarshalErr != nil {
		return Message{}, unmarshalErr
	}

	decoded, decodedErr := base64.StdEncoding.DecodeString(theMsg.BodyString)
	if decodedErr != nil {
		return theMsg, fmt.Errorf("Failed to decode message from base64: %w", decodedErr)
	}
	return WithBody(theMsg, string(decoded)), nil
}

// DeserializeMessageString Deserializes message from string
func DeserializeMessageString(s string) (Message, error) {
	return DeserializeMessageBytes([]byte(s))
}
