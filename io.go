// +build linux

package zmq4chan

import (
	"sync"
	"syscall"

	zmq "github.com/pebbe/zmq4"
)

const (
	epollEventBufferSize = 256 // per IO instance
)

// Data holds a message part which will be sent or has been received.
type Data struct {
	Bytes []byte
	More  bool // true if more parts will be sent/received
}

// String converts r.Bytes to a string.
func (r Data) String() string {
	return string(r.Bytes)
}

// IO implements channel-based messaging.
type IO struct {
	epollFd int
	lock    sync.Mutex
	workers map[int32]*worker
}

// NewIO allocates resources which will not be released before program
// termination.  You shouldn't need many instances.  Panics on error.
func NewIO() (io *IO) {
	epollFd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		panic(err)
	}

	io = &IO{
		epollFd: epollFd,
		workers: make(map[int32]*worker),
	}

	go eventLoop(io)
	return
}

// Add registers a socket for sending and/or receiving.  The caller can't
// access the socket directly after this.  The send channel (if any) should be
// closed by the caller.
func (io *IO) Add(s *zmq.Socket, send <-chan Data, recv chan<- Data) (err error) {
	fd, err := s.GetFd()
	if err != nil {
		return
	}

	w := newWorker()

	io.lock.Lock()
	io.workers[int32(fd)] = w
	io.lock.Unlock()

	defer func() {
		if err != nil {
			io.lock.Lock()
			delete(io.workers, int32(fd))
			io.lock.Unlock()
		}
	}()

	e := &syscall.EpollEvent{
		Events: syscall.EPOLLIN | syscall.EPOLLET&0xffffffff,
		Fd:     int32(fd),
	}

	if err = syscall.EpollCtl(io.epollFd, syscall.EPOLL_CTL_ADD, fd, e); err != nil {
		return
	}

	state, err := s.GetEvents()
	if err != nil {
		syscall.EpollCtl(io.epollFd, syscall.EPOLL_CTL_DEL, fd, nil)
		return
	}

	go workerLoop(io, s, fd, w, send, recv, state)
	return
}

// Remove closes a socket.  If it has been registered, it will be removed.  The
// recv channel (if any) will be closed.
func (io *IO) Remove(s *zmq.Socket) (err error) {
	fd, err := s.GetFd()
	if err != nil {
		return
	}

	io.lock.Lock()
	w := io.workers[int32(fd)]
	if w != nil {
		delete(io.workers, int32(fd))
	}
	io.lock.Unlock()

	if w != nil {
		w.close()
	} else {
		err = s.Close()
	}
	return
}

func eventLoop(io *IO) {
	const mask = syscall.EPOLLIN | syscall.EPOLLERR | syscall.EPOLLHUP

	buf := make([]syscall.EpollEvent, epollEventBufferSize)

	for {
		n, err := syscall.EpollWait(io.epollFd, buf, -1)
		if err != nil {
			panic(err)
		}

		io.lock.Lock()

		for _, e := range buf[:n] {
			if e.Events&mask != 0 {
				if w := io.workers[e.Fd]; w != nil {
					w.notify()
				}
			}
		}

		io.lock.Unlock()
	}
}

type worker struct {
	notifier chan struct{}
	closer   chan struct{}
}

func newWorker() *worker {
	return &worker{
		notifier: make(chan struct{}, 1),
		closer:   make(chan struct{}),
	}
}

func (w *worker) notify() {
	select {
	case w.notifier <- struct{}{}:
	default:
	}
}

func (w *worker) close() {
	close(w.closer)
}

func workerLoop(io *IO, s *zmq.Socket, fd int, w *worker, send <-chan Data, recv chan<- Data, state zmq.State) {
	defer func() {
		if recv != nil {
			close(recv)
		}
		syscall.EpollCtl(io.epollFd, syscall.EPOLL_CTL_DEL, fd, nil)
		s.Close()
	}()

	const (
		fullState = zmq.POLLIN | zmq.POLLOUT
	)

	var (
		sendBuf     Data
		sendPending bool
		recvBuf     Data
		recvPending bool
	)

	for {
		var (
			err error

			sendActive <-chan Data
			recvActive chan<- Data
		)

		if !sendPending {
			sendActive = send
		}

		if recvPending {
			recvActive = recv
		}

		select {
		case <-w.notifier:
			if state&fullState != fullState {
				if state, err = s.GetEvents(); err != nil {
					handleGeneralError(err)
					return
				}
			}

		case <-w.closer:
			return

		case sendBuf, sendPending = <-sendActive:
			if !sendPending {
				send = nil
			}

		case recvActive <- recvBuf:
			recvPending = false
			recvBuf.Bytes = nil
		}

		for {
			loop := false

			if sendPending && state&zmq.POLLOUT != 0 {
				flags := zmq.DONTWAIT

				if sendBuf.More {
					flags |= zmq.SNDMORE
				}

				if _, err = s.SendBytes(sendBuf.Bytes, flags); err == nil {
					sendPending = false
					sendBuf.Bytes = nil
					loop = true
				} else if !handleIOError(err) {
					return
				}

				if state, err = s.GetEvents(); err != nil {
					handleGeneralError(err)
					return
				}
			}

			if !recvPending && state&zmq.POLLIN != 0 {
				if data, err := s.RecvBytes(zmq.DONTWAIT); err == nil {
					if more, err := s.GetRcvmore(); err == nil {
						recvBuf.Bytes = data
						recvBuf.More = more
						recvPending = true
						loop = true
					} else {
						handleGeneralError(err)
						return
					}
				} else if !handleIOError(err) {
					return
				}

				if state, err = s.GetEvents(); err != nil {
					handleGeneralError(err)
					return
				}
			}

			if !loop {
				break
			}
		}
	}
}

func handleGeneralError(err error) {
	switch err {
	case zmq.ErrorSocketClosed, zmq.ErrorContextClosed:
		return
	}

	switch zmq.AsErrno(err) {
	case zmq.ETERM:
		return
	}

	panic(err)
}

func handleIOError(err error) bool {
	switch err {
	case zmq.ErrorSocketClosed, zmq.ErrorContextClosed:
		return false
	}

	switch zmq.AsErrno(err) {
	case zmq.Errno(syscall.EAGAIN):
		return true

	case zmq.ETERM:
		return false
	}

	panic(err)
}
