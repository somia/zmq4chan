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
		zmqchan.Data{[]byte("hello"), true},
		zmqchan.Data{[]byte("world"), false},
		zmqchan.Data{[]byte("123"), true},
		zmqchan.Data{[]byte("4567"), true},
		zmqchan.Data{[]byte("89"), false},
	}
)

func TestSendMessageBytes(t *testing.T) {
	var (
		input  = make(chan [][]byte)
		output = zmqchan.SendMessageBytes(input)
	)

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
		t.Error("too few parts received")
	}
}

func TestRecvMessageBytes(t *testing.T) {
	t.Log("TODO")
}
