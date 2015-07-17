package zmq4chan_test

import (
	"bytes"
	"testing"

	zmqchan "."
)

var (
	messages = [][][]byte{
		[][]byte{
			[]byte("hello"),
			[]byte("world"),
		},
		[][]byte{
			[]byte("123"),
			[]byte("4567"),
			[]byte("89"),
		},
	}

	parts = []zmqchan.Data{
		zmqchan.Data{Bytes: []byte("hello"), More: true},
		zmqchan.Data{Bytes: []byte("world"), More: false},
		zmqchan.Data{Bytes: []byte("123"), More: true},
		zmqchan.Data{Bytes: []byte("4567"), More: true},
		zmqchan.Data{Bytes: []byte("89"), More: false},
	}
)

func TestSendMessageBytes(t *testing.T) {
	input := make(chan [][]byte)
	output := zmqchan.SendMessageBytes(input)

	go func() {
		defer close(input)

		for _, message := range messages {
			input <- message
		}
	}()

	i := 0

	for part := range output {
		if bytes.Compare(part.Bytes, parts[i].Bytes) != 0 {
			t.Errorf("i=%d Bytes mismatch", i)
		}

		if part.More != parts[i].More {
			t.Errorf("i=%d More mismatch", i)
		}

		i++
	}

	if i != len(parts) {
		t.Error("incorrect number of parts received")
	}
}

func TestRecvMessageBytes(t *testing.T) {
	output := make(chan [][]byte)
	input := zmqchan.RecvMessageBytes(output)

	go func() {
		defer close(input)

		for _, part := range parts {
			input <- part
		}
	}()

	i := 0

	for message := range output {
		if len(message) != len(messages[i]) {
			t.Fatalf("i=%d length mismatch", i)
		}

		for j, part := range message {
			if bytes.Compare(part, messages[i][j]) != 0 {
				t.Errorf("i=%d part=%d mismatch", i, j)
			}
		}

		i++
	}

	if i != len(messages) {
		t.Error("incorrect number of messages received")
	}
}
