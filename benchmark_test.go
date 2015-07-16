package zmq4chan_test

import (
	"testing"

	zmq "github.com/pebbe/zmq4"
)

func BenchmarkFunction(b *testing.B) {
	for i := 0; i < b.N; i++ {
		zmq.Type(i).String()
	}
}

func BenchmarkGoroutine(b *testing.B) {
	c := make(chan struct{})

	go func() {
		for range c {
		}
	}()

	defer close(c)

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		c <- struct{}{}
	}

	b.StopTimer()
}

func BenchmarkCGO(b *testing.B) {
	for i := 0; i < b.N; i++ {
		zmq.Version()
	}
}
