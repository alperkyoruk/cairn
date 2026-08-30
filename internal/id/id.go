// Package id generates the identifiers Cairn stores in its TEXT primary keys.
package id

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// New returns a UUIDv7 (RFC 9562): a 48-bit millisecond timestamp followed by
// random bits. Being time-ordered means ids sort by creation and index inserts
// stay local, which is what we want from a primary key stored as text.
func New() string {
	var b [16]byte

	ms := uint64(time.Now().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	// crypto/rand.Read is documented never to return an error.
	rand.Read(b[6:])

	b[6] = (b[6] & 0x0f) | 0x70 // version 7
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
