package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters. Changing them only affects new hashes: old hashes
// carry their own parameters and keep working.
const (
	argonTime = 3
	argonMem  = 64 * 1024 // KiB
	argonPar  = 2
	argonLen  = 32
)

var b64 = base64.RawStdEncoding

// newPassword returns a random 26-character password (130 bits).
func newPassword() string { return rand.Text() }

// hashPassword returns a PHC-format string:
// $argon2id$v=19$m=65536,t=3,p=2$<salt>$<key>
func hashPassword(pw string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMem, argonPar, argonLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMem, argonTime, argonPar, b64.EncodeToString(salt), b64.EncodeToString(key))
}

func checkPassword(hash, pw string) bool {
	p := strings.Split(hash, "$")
	if len(p) != 6 || p[1] != "argon2id" || p[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return false
	}
	var mem, iters uint32
	var par uint8
	if _, err := fmt.Sscanf(p[3], "m=%d,t=%d,p=%d", &mem, &iters, &par); err != nil {
		return false
	}
	if mem == 0 || mem > 1<<20 || iters == 0 || iters > 16 || par == 0 {
		return false
	}
	salt, err1 := b64.DecodeString(p[4])
	want, err2 := b64.DecodeString(p[5])
	if err1 != nil || err2 != nil || len(want) < 16 {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, iters, mem, par, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// newGen returns a random session generation for User.Gen.
func newGen() uint64 {
	var b [8]byte
	rand.Read(b[:])
	return binary.LittleEndian.Uint64(b[:])
}
