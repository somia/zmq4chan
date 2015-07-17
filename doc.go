// Package zmq4chan extends https://github.com/pebbe/zmq4 with channel I/O.
//
// Since ZeroMQ sockets are not thread-safe, they cannot be used directly for
// sending and receiving in different goroutines (e.g. when using a socket type
// with unrestricted send/receive pattern).  This package interleaves the
// ZeroMQ calls safely, while providing a simple API.  The implementation uses
// epoll, which makes it Linux-specific.
//
// Multipart messaging can be achieved by combining the basic IO.Add interface
// with the SendMessageBytes and RecvMessageBytes adapters.
//
// Example:
//
//   io := zmq4chan.NewIO()
//
//   s, err := zmq4.NewSocket(zmq4.REP)
//   defer io.Remove(s)
//
//   recv := make(chan [][]byte)
//   send := make(chan [][]byte)
//   defer close(send)
//
//   err = io.Add(s, zmq4chan.SendMessageBytes(send), zmq4chan.ReceiveMessageBytes(recv))
//
//   for msg := range recv {
//       send <- msg
//   }
//
package zmq4chan
