package zmq4chan

// SendMessageBytes converts messages into individual parts.  The channel
// passed as the parameter should be closed by the caller.
func SendMessageBytes(send <-chan [][]byte) <-chan Data {
	c := make(chan Data)

	go func() {
		defer close(c)

		for message := range send {
			for i, b := range message {
				c <- Data{
					Bytes: b,
					More:  i+1 != len(message),
				}
			}
		}
	}()

	return c
}

// RecvMessageBytes converts individual parts into complete messages.  The
// caller should arrange for the returned channel to be closed (e.g. by
// delegating it to an IO instance).
func RecvMessageBytes(recv chan<- [][]byte) chan<- Data {
	c := make(chan Data)

	var message [][]byte

	go func() {
		defer close(recv)

		for data := range c {
			message = append(message, data.Bytes)
			if !data.More {
				recv <- message
				message = nil
			}
		}
	}()

	return c
}
