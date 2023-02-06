package main

import (
	"crypto/subtle"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"

	"github.com/jakemakesstuff/packetmaker"
	"github.com/webscalesoftwareltd/hypercache/radix"
)

type eventDispatcher struct {
	mu      sync.RWMutex
	writers []io.Writer
}

func (e *eventDispatcher) addWriter(w io.Writer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.writers = append(e.writers, w)
}

func (e *eventDispatcher) removeWriter(w io.Writer) {
	e.mu.Lock()
	defer e.mu.Unlock()

	i := -1
	for possibleIndex, possible := range e.writers {
		if possible == w {
			i = possibleIndex
		}
	}
	if i == -1 {
		return
	}

	e.writers[i] = e.writers[len(e.writers)-1]
	e.writers = e.writers[:len(e.writers)-1]
}

var n4 = []byte{0, 0, 0, 0}

func (e *eventDispatcher) dispatch(event []byte, except io.Writer) {
	event = packetmaker.New().Bytes(n4).
		Uint32(uint32(len(event)), true).Make()
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, v := range e.writers {
		v := v
		if v == except {
			continue
		}
		go v.Write(event)
	}
}

func write(conn net.Conn, b []byte) bool {
	_ = conn.SetWriteDeadline(time.Now().Add(time.Minute))
	_, err := conn.Write(b)
	return err == nil
}

const (
	invalidPacketErr       = "InvalidPacket"
	invalidProtocolMessage = "Invalid header."
)

var invalidProtocolPacket = packetmaker.New().
	Byte(1).
	Byte(uint8(len(invalidPacketErr))).
	String(invalidPacketErr).
	Byte(uint8(len(invalidProtocolMessage))).
	String(invalidProtocolMessage).
	Make()

const (
	invalidCredentialsErr     = "InvalidCredentials"
	invalidCredentialsMessage = "The specified password is invalid."
)

var invalidCredentialsPacket = packetmaker.New().
	Byte(1).
	Byte(uint8(len(invalidCredentialsErr))).
	String(invalidCredentialsErr).
	Byte(uint8(len(invalidCredentialsMessage))).
	String(invalidCredentialsMessage).
	Make()

const (
	dbNotFoundErr     = "DatabaseNotFound"
	dbNotFoundMessage = "The database index is too large for the number of databases in this application."
)

var dbNotFoundPacket = packetmaker.New().
	Byte(1).
	Byte(uint8(len(dbNotFoundErr))).
	String(dbNotFoundErr).
	Byte(uint8(len(dbNotFoundMessage))).
	String(dbNotFoundMessage).
	Make()

func tryUnlock(mu *sync.Mutex) bool {
	defer func() {
		_ = recover()
	}()
	mu.Unlock()
	return true
}

type recordStack struct {
	key, value []byte
	prev       *recordStack
}

func processPacket(
	conn net.Conn, packet []byte, replyId uint32,
	db radix.RadixTree, mu *sync.Mutex,
	dispatcher *eventDispatcher,
) {
	raiseError := func(exception, message string) {
		p := packetmaker.New().
			Uint32(replyId, true).
			Byte(1).
			Byte(uint8(len(exception))).
			String(exception).
			Byte(uint8(len(message))).
			String(message).
			Make()
		write(conn, p)
	}

	returnResult := func(b []byte, writeLen bool) bool {
		p := packetmaker.New().
			Uint32(replyId, true).
			Byte(0)
		if writeLen {
			p.Uint32(uint32(len(b)), true)
		}
		p.Bytes(b)
		return write(conn, p.Make())
	}

	packetLen := len(packet)
	if packetLen == 0 {
		raiseError("InvalidPacket", "No start byte found.")
		return
	}

	switch packet[0] {
	case 0:
		// Pong!
		p := packetmaker.New().
			Uint32(replyId, true).
			Byte(0).
			Make()
		write(conn, p)
	case 1:
		// Record get.
		packet = packet[1:]
		value, deallocator := db.Get(packet)
		defer deallocator()
		if value == nil {
			raiseError("NotFound", "The key was not found in the database.")
			return
		}
		returnResult(value, true)
	case 2:
		// Record delete.
		packet = packet[1:]
		var data []byte
		if db.DeleteKey(packet) {
			data = []byte{1}
		} else {
			data = []byte{0}
		}
		returnResult(data, true)
	case 3:
		// Record set.
		packet = packet[1:]
		if len(packet) < 4 {
			raiseError(
				"InvalidPacket",
				"Key length not specified.")
			return
		}
		keyLen := int(binary.LittleEndian.Uint32(packet))
		packet = packet[4:]
		if len(packet) < keyLen {
			raiseError(
				"InvalidPacket",
				"Packet too short for key length.")
			return
		}
		key := packet[:keyLen]
		packet = packet[keyLen:]
		var data []byte
		if db.Set(key, packet) {
			data = []byte{1}
		} else {
			data = []byte{0}
		}
		returnResult(data, true)
	case 4:
		// Free tree.
		db.FreeTree()
		returnResult([]byte{}, false)
	case 5:
		// Delete prefix.
		b := []byte{0, 0, 0, 0, 0, 0, 0, 0}
		packet = packet[1:]
		res := db.DeletePrefix(packet)
		binary.LittleEndian.PutUint64(b, res)
		returnResult(b, false)
	case 6:
		// Walk prefix.
		packet = packet[1:]
		length := 0
		var stack *recordStack
		freer := &radix.PendingFreer{}
		db.WalkPrefix(packet, func(key, value []byte) bool {
			stack = &recordStack{
				key:   key,
				value: value,
				prev:  stack,
			}
			length++
			return true
		}, freer)
		m := packetmaker.New().Uint32(uint32(length), true)
		for stack != nil {
			m.Uint32(uint32(len(stack.key)), true).
				Bytes(stack.key).
				Uint32(uint32(len(stack.value)), true).
				Bytes(stack.value)
			stack = stack.prev
		}
		p := m.Make()
		returnResult(p, false)
		freer.FreeAll()
	case 7:
		// Mutex lock.
		mu.Lock()
		sent := returnResult([]byte{}, false)
		if !sent {
			// Immediately unlock.
			mu.Unlock()
		}
	case 8:
		// Mutex unlock.
		unlocked := tryUnlock(mu)
		if unlocked {
			returnResult([]byte{}, false)
			return
		}
		raiseError(
			"UnlockError",
			"Mutex was already unlocked.")
	case 9:
		// Event send.
		dispatcher.dispatch(packet, conn)
		returnResult([]byte{}, false)
	default:
		// Unknown byte.
		raiseError("InvalidPacket", "Unknown start byte.")
	}
}

func spawnHnpHandler(conn net.Conn) {
	// Defer closing the connection.
	defer conn.Close()

	// Check the start header.
	startHeader := make([]byte, 8)
	_ = conn.SetReadDeadline(time.Now().Add(time.Second * 2))
	_, err := conn.Read(startHeader)
	if err != nil {
		return
	}
	if startHeader[0] != 'H' ||
		startHeader[1] != 'N' ||
		startHeader[2] != 'P' ||
		startHeader[3] != '1' {
		// Hang up with an exception.
		write(conn, invalidProtocolPacket)
		return
	}

	// Get the password.
	passwordLen := binary.LittleEndian.Uint16(startHeader[len(startHeader)-2:])
	passwordAttempt := make([]byte, passwordLen)
	n, err := conn.Read(passwordAttempt)
	if err != nil {
		return
	}
	if n != int(passwordLen) {
		// Assume connection is dead.
		return
	}
	if subtle.ConstantTimeCompare(passwordAttempt, password) != 1 {
		// Send a invalid credentials error.
		write(conn, invalidCredentialsPacket)
		return
	}

	// Get the DB this connection is for.
	dbIndex := binary.LittleEndian.Uint16(startHeader[4:6])
	if dbIndex >= uint16(len(trees)) {
		// Send a database not found error.
		write(conn, dbNotFoundPacket)
		return
	}
	db := trees[dbIndex]
	mu := &mutexes[dbIndex]

	// Send a null byte. Success!
	write(conn, []byte{0})

	// Add the connection to the event system.
	dispatcher := &eventDispatchers[dbIndex]
	dispatcher.addWriter(conn)
	defer dispatcher.removeWriter(conn)

	startHeader = make([]byte, 8)
	for {
		// Read the start header.
		_ = conn.SetReadDeadline(time.Now().Add(time.Second * 2))
		n, err = conn.Read(startHeader)
		if err != nil || n != 8 {
			return
		}

		// Make sure the reply ID isn't 0.
		replyId := binary.LittleEndian.Uint32(startHeader)
		if replyId == 0 {
			return
		}

		// Get the packet length.
		packetLen := binary.LittleEndian.Uint32(startHeader[4:])
		if packetLen == 0 {
			// Malformed packet.
			return
		}

		// Allocate memory for the packet and fetch it.
		packet := make([]byte, packetLen)
		_ = conn.SetReadDeadline(time.Now().Add(time.Second * 2))
		n, err := conn.Read(packet)
		if err != nil {
			return
		}
		if n != int(packetLen) {
			// Malformation risk! Return here.
			return
		}

		// Process the packet.
		go processPacket(conn, packet, replyId, db, mu, dispatcher)
	}
}
