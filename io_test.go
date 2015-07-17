package zmq4chan_test

import (
	"fmt"
	"strconv"
	"testing"

	zmq "github.com/pebbe/zmq4"

	zmqchan "."
)

var io = zmqchan.NewIO()

func TestIO(t *testing.T) {
	const (
		addr1 = "tcp://127.0.0.1:12345"
		addr2 = "tcp://127.0.0.1:12346"
	)

	ctx, err := zmq.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Term()

	go func() {
		s, err := ctx.NewSocket(zmq.REQ)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()

		if err := s.Connect(addr1); err != nil {
			t.Fatal(err)
		}

		if err := s.Connect(addr2); err != nil {
			t.Fatal(err)
		}

		for {
			t.Logf("0 sending")

			if _, err := s.SendMessage("hello", "world"); err != nil {
				t.Logf("0 %s", err)
				return
			}

			t.Logf("0 receiving")

			msg, err := s.RecvMessage(0)
			if err != nil {
				t.Logf("0 %s", err)
				return
			}

			t.Logf("0 %s", msg)
		}
	}()

	ctx1, err := zmq.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx1.Term()

	ctx2, err := zmq.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer ctx2.Term()

	s1, err := ctx1.NewSocket(zmq.REP)
	if err != nil {
		t.Fatal(err)
	}
	defer io.Remove(s1)

	s2, err := ctx2.NewSocket(zmq.REP)
	if err != nil {
		t.Fatal(err)
	}
	defer io.Remove(s2)

	if err := s1.Bind(addr1); err != nil {
		t.Fatal(err)
	}

	if err := s2.Bind(addr2); err != nil {
		t.Fatal(err)
	}

	send1 := make(chan zmqchan.Data)
	recv1 := make(chan zmqchan.Data)

	if err := io.Add(s1, send1, recv1); err != nil {
		t.Fatal(err)
	}

	send2 := make(chan zmqchan.Data)
	recv2 := make(chan zmqchan.Data)

	if err := io.Add(s2, send2, recv2); err != nil {
		t.Fatal(err)
	}

	var (
		send1buf zmqchan.Data
		send2buf zmqchan.Data

		recv1count = 0
		recv2count = 0
	)

	for recv1 != nil || send1 != nil || recv2 != nil || send2 != nil {
		var (
			recv1chan <-chan zmqchan.Data
			recv2chan <-chan zmqchan.Data

			send1chan chan<- zmqchan.Data
			send2chan chan<- zmqchan.Data
		)

		if send1buf.Bytes == nil {
			recv1chan = recv1
		} else {
			send1chan = send1
		}

		if send2buf.Bytes == nil {
			recv2chan = recv2
		} else {
			send2chan = send2
		}

		select {
		case data, ok := <-recv1chan:
			if !ok {
				t.Fatal("1 closed")
			}

			t.Logf("1 '%s'", data)

			if !data.More {
				if recv1count++; recv1count == 10 {
					t.Logf("1 no more")
					recv1 = nil
				}

				send1buf.Bytes = []byte("okie")
			}

		case send1chan <- send1buf:
			send1buf.Bytes = nil

			if recv1 == nil {
				close(send1)
				send1 = nil
			}

		case b, ok := <-recv2chan:
			if !ok {
				t.Fatal("2 closed")
			}

			t.Logf("2 '%s'", b)

			if !b.More {
				if recv2count++; recv2count == 10 {
					t.Logf("2 no more")
					recv2 = nil
				}

				send2buf.Bytes = []byte("dokie")
			}

		case send2chan <- send2buf:
			send2buf.Bytes = nil

			if recv2 == nil {
				close(send2)
				send2 = nil
			}
		}
	}
}

func TestLotsOfIO(t *testing.T) {
	const (
		numSockets = 503
	)

	defer zmq.Term()

	done := make(chan struct{})

	for i := 0; i < numSockets; i++ {
		addr := fmt.Sprintf("inproc://%d", i)

		numMessages := i * 11
		if i&1 == 1 {
			numMessages -= 1317
		}

		go func(addr string, numMessages int) {
			defer func() {
				done <- struct{}{}
			}()

			s, err := zmq.NewSocket(zmq.PULL)
			if err != nil {
				t.Fatal(err)
			}
			defer io.Remove(s)

			if err := s.Bind(addr); err != nil {
				t.Fatal(err)
			}

			c := make(chan zmqchan.Data)

			if err := io.Add(s, nil, c); err != nil {
				t.Fatal(err)
			}

			for n := 0; n < numMessages; n++ {
				m := <-c

				ms, err := strconv.Atoi(m.String())
				if err != nil {
					t.Fatal(err)
				}

				if ms != n {
					t.Fatalf("%d: %d != %d", i, ms, n)
				}
			}
		}(addr, numMessages)

		go func(addr string, numMessages int) {
			defer func() {
				done <- struct{}{}
			}()

			s, err := zmq.NewSocket(zmq.PUSH)
			if err != nil {
				t.Fatal(err)
			}
			defer io.Remove(s)

			if err := s.Connect(addr); err != nil {
				t.Fatal(err)
			}

			c := make(chan zmqchan.Data)
			defer close(c)

			if err := io.Add(s, c, nil); err != nil {
				t.Fatal(err)
			}

			for n := 0; n < numMessages; n++ {
				c <- zmqchan.Data{
					Bytes: []byte(strconv.Itoa(n)),
				}
			}
		}(addr, numMessages)
	}

	for i := 0; i < numSockets; i++ {
		<-done
	}
}
