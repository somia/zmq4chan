// +build linux

package zmq4chan

import (
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	zmq "github.com/pebbe/zmq4"
)

const (
	// epollEventBufferSize per IO instance.
	epollEventBufferSize = 1000

	// epollet is unsigned syscall.EPOLLET.
	epollet = 0x80000000
)

// Data holds a message part which will be sent or has been received.
type Data struct {
	Bytes []byte // Bytes is non-nil.
	More  bool   // More is true if more parts will be sent/received.
}

// String converts r.Bytes to a string.
func (r Data) String() string {
	return string(r.Bytes)
}

// IO implements channel-based messaging.
type IO struct {
	epollFd int
	lock    sync.Mutex
	workers map[*zmq.Socket]*worker
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
		workers: make(map[*zmq.Socket]*worker),
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

	var (
		w = &worker{
			notifier: make(chan struct{}, 1),
		}
		epollEvent = &syscall.EpollEvent{
			Events: syscall.EPOLLIN | epollet,
		}
	)

	setEpollDataPtr(epollEvent, uintptr(unsafe.Pointer(w)))

	if err = syscall.EpollCtl(io.epollFd, syscall.EPOLL_CTL_ADD, fd, epollEvent); err != nil {
		return
	}

	state, err := s.GetEvents()
	if err != nil {
		syscall.EpollCtl(io.epollFd, syscall.EPOLL_CTL_DEL, fd, nil)
		return
	}

	io.lock.Lock()
	io.workers[s] = w
	io.lock.Unlock()

	go workerLoop(io, s, fd, w, send, recv, state)
	return
}

// Remove closes a socket.  If it has been registered, it will be removed in
// orderly fashion.
func (io *IO) Remove(s *zmq.Socket) {
	io.lock.Lock()
	w := io.workers[s]
	if w != nil {
		delete(io.workers, s)
	}
	io.lock.Unlock()

	if w != nil {
		atomic.StoreInt32(&w.closed, 1)
		w.notify()
	} else {
		s.Close()
	}
}

func eventLoop(io *IO) {
	epollEvents := make([]syscall.EpollEvent, epollEventBufferSize)

	for {
		n, err := syscall.EpollWait(io.epollFd, epollEvents, -1)
		if err != nil {
			return
		}

		for i := 0; i < n; i++ {
			e := &epollEvents[i]
			w := (*worker)(unsafe.Pointer(getEpollDataPtr(e)))

			if e.Events&syscall.EPOLLERR != 0 {
				panic("epoll error event")
			}

			if e.Events&syscall.EPOLLIN != 0 {
				w.notify()
			}
		}
	}
}

type worker struct {
	notifier chan struct{}
	closed   int32
}

func (w *worker) notify() {
	select {
	case w.notifier <- struct{}{}:
	default:
	}
}

func workerLoop(io *IO, s *zmq.Socket, fd int, w *worker, send <-chan Data, recv chan<- Data, state zmq.State) {
	defer func() {
		if recv != nil {
			close(recv)
		}
		syscall.EpollCtl(io.epollFd, syscall.EPOLL_CTL_DEL, fd, nil)
		s.Close()
	}()

	var (
		sendBuf Data
		recvBuf Data
	)

	for {
		var (
			err error
			ok  bool

			sendChan <-chan Data
			recvChan chan<- Data
		)

		if send != nil && sendBuf.Bytes == nil {
			sendChan = send
		}

		if recv != nil && recvBuf.Bytes != nil {
			recvChan = recv
		}

		select {
		case <-w.notifier:
			if atomic.LoadInt32(&w.closed) != 0 {
				return
			}

			if state, err = s.GetEvents(); err != nil {
				handleGeneralError(err)
				return
			}

		case sendBuf, ok = <-sendChan:
			if !ok {
				send = nil
			}

		case recvChan <- recvBuf:
			recvBuf.Bytes = nil
		}

		for {
			retry := false

			if sendBuf.Bytes != nil && state&zmq.POLLOUT != 0 {
				flags := zmq.DONTWAIT

				if sendBuf.More {
					flags |= zmq.SNDMORE
				}

				if _, err = s.SendBytes(sendBuf.Bytes, flags); err == nil {
					sendBuf.Bytes = nil
					retry = true
				} else if !handleIOError(err) {
					return
				}

				if state, err = s.GetEvents(); err != nil {
					handleGeneralError(err)
					return
				}
			}

			if recvBuf.Bytes == nil && state&zmq.POLLIN != 0 {
				if data, err := s.RecvBytes(zmq.DONTWAIT); err == nil {
					if more, err := s.GetRcvmore(); err == nil {
						recvBuf.Bytes = data
						recvBuf.More = more
						retry = true
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

			if !retry {
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
