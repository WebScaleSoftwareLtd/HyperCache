package hypercache

import (
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jakemakesstuff/packetmaker"
)

type hnpConn struct {
	c net.Conn

	replyAtom uint32

	replies   map[uint32]func(error)
	repliesMu sync.Mutex

	events   []chan []byte
	eventsMu sync.RWMutex

	lastErr   error
	lastErrMu sync.RWMutex
}

// AddEventHandler is used to add a handler for custom events.
// Note that the bytes should not be mutated.
func (h *hnpConn) AddEventHandler(ch chan []byte) {
	h.eventsMu.Lock()
	h.events = append(h.events, ch)
	h.eventsMu.Unlock()
}

func (h *hnpConn) getConnectionError() error {
	h.lastErrMu.RLock()
	err := h.lastErr
	h.lastErrMu.RUnlock()
	return err
}

// Ping is used to ping the server.
func (h *hnpConn) Ping() error {
	// Get the reply ID.
	replyId := h.replyId()

	// Lock the replies map.
	h.repliesMu.Lock()

	// Check if there was a connection error.
	err := h.getConnectionError()
	if err != nil {
		h.repliesMu.Unlock()
		return err
	}

	// Defines the error channel.
	errorCh := make(chan error, 1)
	b := packetmaker.New().
		Uint32(replyId, true).
		Uint32(1, true).
		Byte(0).
		Make()
	h.replies[replyId] = func(err error) { errorCh <- err }
	h.repliesMu.Unlock()
	_, err = h.c.Write(b)
	if err != nil {
		return err
	}

	// Return any errors.
	return <-errorCh
}

// Get is used to get a record.
func (h *hnpConn) Get(key []byte) ([]byte, error) {
	// Get the reply ID.
	replyId := h.replyId()

	// Lock the replies map.
	h.repliesMu.Lock()

	// Check if there was a connection error.
	err := h.getConnectionError()
	if err != nil {
		h.repliesMu.Unlock()
		return nil, err
	}

	// Defines the error channel.
	errorCh := make(chan error, 1)
	b := packetmaker.New().
		Uint32(replyId, true).
		Uint32(uint32(len(key)+1), true).
		Byte(1).
		Bytes(key).
		Make()
	var value []byte
	h.replies[replyId] = func(err error) {
		// Handle if the error isn't nil.
		if err != nil {
			errorCh <- err
			return
		}

		// Use the first 4 bytes of the old packet.
		fb := b[:4]
		_, err = h.c.Read(fb)
		if err != nil {
			errorCh <- err
			return
		}

		// Get it as an uint32.
		packetLen := binary.LittleEndian.Uint32(fb)

		// Consume these bytes.
		value = make([]byte, packetLen)
		_, err = h.c.Read(value)
		errorCh <- err
	}
	h.repliesMu.Unlock()
	_, err = h.c.Write(b)
	if err != nil {
		return nil, err
	}

	// Return any errors.
	err = <-errorCh
	return value, err
}

// MutexLock is used to lock a global mutex.
func (h *hnpConn) MutexLock() error {
	// Get the reply ID.
	replyId := h.replyId()

	// Lock the replies map.
	h.repliesMu.Lock()

	// Check if there was a connection error.
	err := h.getConnectionError()
	if err != nil {
		h.repliesMu.Unlock()
		return err
	}

	// Defines the error channel.
	errorCh := make(chan error, 1)
	b := packetmaker.New().
		Uint32(replyId, true).
		Uint32(1, true).
		Byte(7).
		Make()
	h.replies[replyId] = func(err error) { errorCh <- err }
	h.repliesMu.Unlock()
	_, err = h.c.Write(b)
	if err != nil {
		return err
	}

	// Return any errors.
	return <-errorCh
}

// MutexUnlock is used to unlock a globally locked mutex.
func (h *hnpConn) MutexUnlock() error {
	// Get the reply ID.
	replyId := h.replyId()

	// Lock the replies map.
	h.repliesMu.Lock()

	// Check if there was a connection error.
	err := h.getConnectionError()
	if err != nil {
		h.repliesMu.Unlock()
		return err
	}

	// Defines the error channel.
	errorCh := make(chan error, 1)
	b := packetmaker.New().
		Uint32(replyId, true).
		Uint32(1, true).
		Byte(8).
		Make()
	h.replies[replyId] = func(err error) { errorCh <- err }
	h.repliesMu.Unlock()
	_, err = h.c.Write(b)
	if err != nil {
		return err
	}

	// Return any errors.
	return <-errorCh
}

// SendEvent is used to send an event to the HyperCache server.
func (h *hnpConn) SendEvent(b []byte) error {
	// Get the reply ID.
	replyId := h.replyId()

	// Lock the replies map.
	h.repliesMu.Lock()

	// Check if there was a connection error.
	err := h.getConnectionError()
	if err != nil {
		h.repliesMu.Unlock()
		return err
	}

	// Defines the error channel.
	errorCh := make(chan error, 1)
	b = packetmaker.New().
		Uint32(replyId, true).
		Uint32(uint32(len(b)+1), true).
		Byte(9).
		Bytes(b).
		Make()
	h.replies[replyId] = func(err error) { errorCh <- err }
	h.repliesMu.Unlock()
	_, err = h.c.Write(b)
	if err != nil {
		return err
	}

	// Return any errors.
	return <-errorCh
}

func (h *hnpConn) throwError(err error) {
	h.lastErrMu.Lock()
	h.lastErr = err
	h.lastErrMu.Unlock()

	h.repliesMu.Lock()
	m := h.replies
	h.replies = map[uint32]func(error){}
	for _, v := range m {
		v(err)
	}
	h.repliesMu.Unlock()
}

func (h *hnpConn) replyId() uint32 {
	return atomic.AddUint32(&h.replyAtom, 1) + 1
}

func (h *hnpConn) getException() error {
	b := make([]byte, 512)
	ob := b[:1]
	_ = h.c.SetWriteDeadline(time.Now().Add(time.Second * 2))
	_, err := h.c.Read(ob)
	if err != nil {
		return err
	}
	_ = h.c.SetWriteDeadline(time.Now().Add(time.Second * 2))
	exceptionNameB := b[:ob[0]]
	_, err = h.c.Read(exceptionNameB)
	if err != nil {
		return err
	}
	exceptionName := string(exceptionNameB)
	_ = h.c.SetWriteDeadline(time.Now().Add(time.Second * 2))
	_, err = h.c.Read(ob)
	if err != nil {
		return err
	}
	exceptionDescriptionB := b[:ob[0]]
	_, err = h.c.Read(exceptionDescriptionB)
	if err != nil {
		return err
	}

	return toException(exceptionName, exceptionDescriptionB)
}

func (h *hnpConn) readLoop() {
	fb := make([]byte, 5)
	for {
		// Read the contents.
		_, err := h.c.Read(fb)
		if err != nil {
			h.throwError(err)
			return
		}

		// Get the reply ID.
		replyId := binary.LittleEndian.Uint32(fb)
		if replyId == 0 {
			// Check the next byte is 0. If not, this is a unsupported packet.
			if fb[4] == 0 {
				// Get the length of the event.
				four := fb[:4]

				// Read the length.
				_, err = h.c.Read(four)
				if err != nil {
					h.throwError(err)
					return
				}

				// Allocate and read the number of bytes specified.
				eventLen := binary.LittleEndian.Uint32(four)
				event := make([]byte, eventLen)
				_, err = h.c.Read(event)
				if err != nil {
					h.throwError(err)
					return
				}

				// Send it to each channel.
				h.eventsMu.RLock()
				for _, v := range h.events {
					select {
					case v <- event:
					default:
					}
				}
				h.eventsMu.RUnlock()
			}
		} else {
			// Check if this is an exception.
			isException := fb[4] == 1
			var err error
			if isException {
				err = h.getException()
			}
			h.repliesMu.Lock()
			hn, ok := h.replies[replyId]
			delete(h.replies, replyId)
			h.repliesMu.Unlock()
			if ok {
				hn(err)
			}
		}
	}
}

// NewConnectionWithHNPSocket is used to connect with a newly made HNP socket.
func NewConnectionWithHNPSocket(c net.Conn, password string, db uint16) (HNPImplementation, error) {
	h := &hnpConn{
		c:       c,
		replies: map[uint32]func(error){},
	}

	// Do the initial handshake.
	_ = c.SetWriteDeadline(time.Now().Add(time.Second * 2))
	b := packetmaker.New().
		String("HNP1").
		Uint16(db, true).
		Uint16(uint16(len(password)), true).
		String(password).
		Make()
	_, err := c.Write(b)
	if err != nil {
		return nil, err
	}
	_ = c.SetReadDeadline(time.Now().Add(time.Minute))
	ob := []byte{0}
	_, err = c.Read(ob)
	if err != nil {
		return nil, err
	}

	// Check if the byte is 1.
	if ob[0] == 1 {
		err = h.getException()
		_ = c.Close()
		return nil, err
	}

	// Start the read loop.
	go h.readLoop()

	// Return the HNP handler.
	return h, nil
}

// NewConnectionWithHNPAddr is used to connect with a HNP address.
func NewConnectionWithHNPAddr(addr, password string, db uint16) (HNPImplementation, error) {
	x, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return NewConnectionWithHNPSocket(x, password, db)
}
