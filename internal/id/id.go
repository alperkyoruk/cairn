// Package id generates the identifiers Cairn stores in its TEXT primary keys.
package id

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// UUIDv7 (RFC 9562) carries a 48-bit millisecond timestamp, so ids sort by
// creation and index inserts stay local. Cairn leans on that ordering harder
// than the timestamp alone can bear: the board sorts by updated_at and breaks
// ties on id, and two tasks touched in the same millisecond are common. A
// purely random tail would order those two rows differently on every query.
//
// So the 12 bits after the version hold a counter rather than randomness --
// the monotonic method described in RFC 9562 section 6.2. Within a
// millisecond ids strictly increase; across milliseconds the timestamp does
// the work. The remaining 62 bits stay random.
var (
	mu     sync.Mutex
	lastMS uint64
	seq    uint16
)

// New returns a fresh identifier. It is safe for concurrent use, and two calls
// never return ids that sort equal or out of order, however close together.
func New() string {
	ms, counter := next()

	var b [16]byte
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	// crypto/rand.Read is documented never to return an error.
	rand.Read(b[6:])

	b[6] = 0x70 | byte(counter>>8)&0x0f // version 7, then the counter's high nibble
	b[7] = byte(counter)
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10

	var s [36]byte
	hex.Encode(s[0:8], b[0:4])
	s[8] = '-'
	hex.Encode(s[9:13], b[4:6])
	s[13] = '-'
	hex.Encode(s[14:18], b[6:8])
	s[18] = '-'
	hex.Encode(s[19:23], b[8:10])
	s[23] = '-'
	hex.Encode(s[24:36], b[10:16])
	return string(s[:])
}

// next returns a timestamp and counter that are strictly greater than the last
// pair issued, even if the clock repeats a millisecond or steps backwards.
func next() (uint64, uint16) {
	mu.Lock()
	defer mu.Unlock()

	if ms := uint64(time.Now().UnixMilli()); ms > lastMS {
		lastMS, seq = ms, 0
		return lastMS, seq
	}

	// Same millisecond, or a clock that moved backwards. Keep counting inside
	// the last millisecond we issued rather than emitting an id that sorts
	// before one already handed out.
	seq++
	if seq > 0x0fff {
		lastMS++
		seq = 0
	}
	return lastMS, seq
}
