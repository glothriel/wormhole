package messages

import "encoding/json"

// SerializeBytes serializes the message for transit over the wire
func SerializeBytes(m Message) []byte {
	b, marshalErr := json.Marshal(m)
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

	return theMsg, nil
}

// DeserializeMessageString Deserializes message from string
func DeserializeMessageString(s string) (Message, error) {
	return DeserializeMessageBytes([]byte(s))
}
