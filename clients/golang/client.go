package hypercache

import (
	"github.com/jakemakesstuff/packetmaker"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type hnpConn struct {
	c net.Conn

	replyAtom uint32

	replies   map[uint32]func(error)
	repliesMu sync.Mutex
}

func (h *hnpConn) replyId() uint32 {
	return atomic.AddUint32(&h.replyAtom, 1) + 1
}

func (h *hnpConn) setReplyHandler(replyId uint32, f func(error)) {
	h.repliesMu.Lock()
	h.replies[replyId] = f
	h.repliesMu.Unlock()
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

func newHnpConn(c net.Conn, db uint16, password string) error {
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
		return err
	}
	_ = c.SetReadDeadline(time.Now().Add(time.Minute))
	ob := []byte{0}
	_, err = c.Read(ob)
	if err != nil {
		return err
	}

	// Check if the byte is 1.
	if ob[0] == 1 {
		err = h.getException()
		_ = c.Close()
		return err
	}

}
