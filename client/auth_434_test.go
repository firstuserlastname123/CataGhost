package client

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func realmFixture434() []byte {
	b := []byte{0, 0, 0, 0, 2, 0}
	// First realm carries optional build data; second must remain aligned.
	b = append(b, 0, 0, 4)
	b = append(b, []byte("First\x00localhost:9000\x00")...)
	b = append(b, 0, 0, 0, 0, 0, 1, 73, 4, 3, 4, 0xeb, 0x3c)
	b = append(b, 0, 0, 0)
	b = append(b, []byte("Second\x00localhost:9001\x00")...)
	return append(b, 0, 0, 0, 0, 0, 1, 99, 0x10, 0)
}

func TestRealm434IDAndOptionalBuild(t *testing.T) {
	b := realmFixture434()
	realms, err := parseRealms434(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(realms) != 2 || realms[0].ID != 73 || realms[1].ID != 99 || realms[1].Name != "Second" || realms[1].Address != "localhost:9001" {
		t.Fatalf("bad realms %+v", realms)
	}
	for i := 0; i < len(b); i++ {
		if _, err := parseRealms434(b[:i]); err == nil {
			t.Fatalf("accepted truncation at %d", i)
		}
	}
	if _, err := parseRealms434(append(b, 0)); err == nil {
		t.Fatal("accepted trailing data")
	}
}

func TestAuth434ChallengeIdentity(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	left.SetDeadline(time.Now().Add(3 * time.Second))
	right.SetDeadline(time.Now().Add(3 * time.Second))
	a := NewAuthClient("account", "synthetic")
	a.conn = left
	done := make(chan error, 1)
	go func() { done <- a.sendLogonChallenge() }()
	wire := make([]byte, 41)
	if _, err := io.ReadFull(right, wire); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire[:13], []byte{0, 8, 37, 0, 0, 'W', 'o', 'W', 4, 3, 4, 0xeb, 0x3c}) || wire[33] != 7 || string(wire[34:]) != "ACCOUNT" {
		t.Fatalf("identity mismatch %x", wire)
	}
}

func TestRealm434RequestWire(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	left.SetDeadline(time.Now().Add(3 * time.Second))
	right.SetDeadline(time.Now().Add(3 * time.Second))
	a := NewAuthClient("account", "synthetic")
	a.conn = left
	done := make(chan error, 1)
	go func() {
		realms, err := a.requestRealmList()
		if err == nil && (len(realms) != 2 || realms[0].ID != 73) {
			err = io.ErrUnexpectedEOF
		}
		done <- err
	}()
	var request [5]byte
	if _, err := io.ReadFull(right, request[:]); err != nil {
		t.Fatal(err)
	}
	if request != [5]byte{0x10, 0, 0x10, 0, 0} {
		t.Fatalf("realm request changed: %x", request)
	}
	b := realmFixture434()
	packet := []byte{0x10}
	packet = binary.LittleEndian.AppendUint16(packet, uint16(len(b)))
	packet = append(packet, b...)
	if err := write434(right, packet); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
