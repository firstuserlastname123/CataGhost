package client

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func key434() []byte {
	b := make([]byte, 40)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func TestSession434Fixture(t *testing.T) {
	// Digest independently computed with .NET SHA1: ACCOUNT || 00000000 ||
	// 01020304 || 05060708 || 00..27 = 6c37b9ff2db829cc48fbe4f212e041e0ae8a94c2.
	// Wire sequence transcribed from TCPP AuthenticationPackets.cpp, not the encoder.
	want, err := hex.DecodeString("000000000000000000e49412b80300000000000000e0fbc22dccaeffeb3c4878563412018a296c37f201020304b90000000041e00000000000384143434f554e54")
	if err != nil {
		t.Fatal(err)
	}
	got := session434("ACCOUNT", key434(), 0x12345678, []byte{1, 2, 3, 4}, []byte{5, 6, 7, 8})
	if !bytes.Equal(got, want) {
		t.Fatalf("auth-session fixture mismatch\ngot  %x\nwant %x", got, want)
	}
	for _, n := range []int{1, 31, 32, 255} {
		got = session434(strings.Repeat("A", n), key434(), 7, make([]byte, 4), make([]byte, 4))
		bits := binary.BigEndian.Uint16(got[56:58])
		if bits>>3 != uint16(n) || bits&7 != 0 || bits&0x8000 != 0 || len(got) != 58+n {
			t.Fatalf("account bits for length %d: %x", n, bits)
		}
	}
}

func success434() []byte { b := make([]byte, 17); b[0] = 0x40; b[16] = 12; return b }

func TestResponse434(t *testing.T) {
	queued := append([]byte(nil), success434()...)
	queued[0] = 0xa0
	queued = append(queued, 7, 0, 0, 0)
	for _, tc := range []struct {
		name      string
		data      []byte
		errorText string
	}{
		{"success", success434(), ""}, {"queue", queued, "queued: position 7"},
		{"reject", []byte{0, 13}, "result 13"}, {"missing-info", []byte{0, 12}, "missing account"},
		{"empty", nil, "truncated"}, {"old-layout", []byte{12}, "truncated"},
		{"extra", append(success434(), 0), "length"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := response434(tc.data)
			if tc.errorText == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.errorText) {
				t.Fatalf("got %v", err)
			}
		})
	}
	for n := 0; n < len(success434()); n++ {
		if response434(success434()[:n]) == nil {
			t.Fatalf("accepted truncation %d", n)
		}
	}
}

// Independently model the server's outbound cryptography, including its direction
// constant. Never use receiveCrypt434 to generate mock-server traffic.
func serverCrypt434() *rc4.Cipher {
	seed, _ := hex.DecodeString("cc98ae04e897eaca12ddc09342915357")
	h := hmac.New(sha1.New, seed)
	h.Write(key434())
	c, _ := rc4.NewCipher(h.Sum(nil))
	drop := make([]byte, 1024)
	c.XORKeyStream(drop, drop)
	return c
}

func frame434(op uint16, b []byte, c *rc4.Cipher) []byte {
	n := len(b) + 2
	h := []byte{byte(n >> 8), byte(n), byte(op), byte(op >> 8)}
	if n > 0x7fff {
		h = []byte{0x80 | byte(n>>16), byte(n >> 8), byte(n), byte(op), byte(op >> 8)}
	}
	if c != nil {
		c.XORKeyStream(h, h)
	}
	return append(h, b...)
}

type fragment434 struct{ io.Reader }

func (r fragment434) Read(b []byte) (int, error) {
	if len(b) > 1 {
		b = b[:1]
	}
	return r.Reader.Read(b)
}

func TestRead434EncryptionAndFraming(t *testing.T) {
	c := serverCrypt434()
	big := bytes.Repeat([]byte{0xa5}, 0x8000)
	wire := append(frame434(0x1234, big, c), frame434(cataAuthResponse, success434(), c)...)
	r := fragment434{bytes.NewReader(wire)}
	decrypt := receiveCrypt434(key434())
	op, b, err := read434(r, decrypt)
	if err != nil || op != 0x1234 || !bytes.Equal(b, big) {
		t.Fatalf("large encrypted packet: %x %v", op, err)
	}
	op, b, err = read434(r, decrypt)
	if err != nil || op != cataAuthResponse || !bytes.Equal(b, success434()) {
		t.Fatalf("continuous cipher/header-only encryption: %x %v", op, err)
	}
	for _, bad := range [][]byte{{0, 1, 0, 0}, {0x90, 0, 1, 0, 0}, {0, 3, 0, 0}, {0x80, 0, 2, 0}} {
		if _, _, err := read434(bytes.NewReader(bad), nil); err == nil {
			t.Fatalf("accepted bad frame %x", bad)
		}
	}
}

type shortWriter434 struct{ bytes.Buffer }

func (w *shortWriter434) Write(b []byte) (int, error) {
	if len(b) > 1 {
		b = b[:1]
	}
	return w.Buffer.Write(b)
}

type zeroWriter434 struct{}

func (zeroWriter434) Write([]byte) (int, error) { return 0, nil }
func TestWrite434(t *testing.T) {
	w := &shortWriter434{}
	if err := write434(w, []byte("hello")); err != nil || w.String() != "hello" {
		t.Fatal(err)
	}
	if write434(zeroWriter434{}, []byte{1}) != io.ErrShortWrite {
		t.Fatal("missing short write error")
	}
}

func TestWorldAuth434Handshake(t *testing.T) {
	// Real loopback listener is a synthetic server only: no QA endpoint or database.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan error, 1)
	go func() { done <- mockWorld434(ln) }()
	err = AuthenticateWorld434(context.Background(), "account", key434(), RealmInfo{ID: 73, Address: ln.Addr().String()})
	if err != nil {
		t.Error(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("mock server did not finish")
	}
}

func mockWorld434(ln net.Listener) error {
	return mockWorldThen434(ln, nil)
}

func mockWorldThen434(ln net.Listener, after func(net.Conn, *rc4.Cipher) error) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	banner := append([]byte{0, byte(len(cataServerBanner))}, cataServerBanner...)
	for _, b := range banner {
		if err := write434(conn, []byte{b}); err != nil {
			return err
		}
	}
	got := make([]byte, 2+len(cataClientBanner))
	if _, err := io.ReadFull(conn, got); err != nil {
		return err
	}
	if !bytes.Equal(got, append([]byte{0, byte(len(cataClientBanner))}, cataClientBanner...)) {
		return fmt.Errorf("bad client initializer")
	}
	challenge := append(bytes.Repeat([]byte{0xdd}, 32), 5, 6, 7, 8, 1)
	if err := write434(conn, frame434(cataAuthChallenge, challenge, nil)); err != nil {
		return err
	}
	var hdr [6]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return err
	}
	if binary.LittleEndian.Uint32(hdr[2:]) != cataAuthSession {
		return fmt.Errorf("auth-session opcode/header is not plaintext")
	}
	n := int(binary.BigEndian.Uint16(hdr[:2])) - 4
	if n != 65 {
		return fmt.Errorf("auth-session size %d", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(conn, body); err != nil {
		return err
	}
	if binary.LittleEndian.Uint16(body[28:30]) != 15595 || binary.LittleEndian.Uint32(body[31:35]) != 73 || string(body[58:]) != "ACCOUNT" {
		return fmt.Errorf("wrong build, realm ID or account")
	}
	// Reassemble digest using server-reader offsets, then verify the random seed.
	var digest [20]byte
	positions := []int{38, 39, 45, 27, 24, 12, 37, 25, 30, 22, 9, 40, 11, 51, 50, 21, 26, 36, 10, 23}
	for i, pos := range positions {
		digest[i] = body[pos]
	}
	h := sha1.New()
	h.Write([]byte("ACCOUNT"))
	h.Write(make([]byte, 4))
	h.Write(body[41:45])
	h.Write([]byte{5, 6, 7, 8})
	h.Write(key434())
	if !bytes.Equal(h.Sum(nil), digest[:]) {
		return fmt.Errorf("server rejected proof")
	}
	c := serverCrypt434()
	// Coalesce an unsolicited packet and auth response, testing cipher continuity.
	if err := write434(conn, append(frame434(0x7777, []byte{9, 8, 7}, c), frame434(cataAuthResponse, success434(), c)...)); err != nil {
		return err
	}
	if after != nil {
		if err := after(conn, c); err != nil {
			return err
		}
	}
	var extra [1]byte
	n, err = conn.Read(extra[:])
	if n != 0 || err != io.EOF {
		return fmt.Errorf("expected clean close without gameplay packet, got %d bytes / %v", n, err)
	}
	return nil
}

func TestWorldAuth434InputValidation(t *testing.T) {
	for _, tc := range []struct {
		user string
		key  []byte
	}{{"", key434()}, {strings.Repeat("A", 256), key434()}, {"A\x00B", key434()}, {"ACCOUNT", nil}} {
		if AuthenticateWorld434(context.Background(), tc.user, tc.key, RealmInfo{}) == nil {
			t.Fatal("invalid input accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if AuthenticateWorld434(ctx, "ACCOUNT", key434(), RealmInfo{Address: "127.0.0.1:0"}) == nil {
		t.Fatal("cancelled dial succeeded")
	}
}

func TestWorldAuth434CancellationWhileReading(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- AuthenticateWorld434(ctx, "ACCOUNT", key434(), RealmInfo{Address: ln.Addr().String()}) }()
	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cancel() // Server deliberately never sends a banner.
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled handshake succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not unblock read")
	}
}

func TestWorldAuth434RejectsBadInitializers(t *testing.T) {
	for _, banner := range [][]byte{{0, 1}, append([]byte{0, byte(len(cataServerBanner))}, bytes.Repeat([]byte{'X'}, len(cataServerBanner))...)} {
		left, right := net.Pipe()
		left.SetDeadline(time.Now().Add(time.Second))
		right.SetDeadline(time.Now().Add(time.Second))
		done := make(chan error, 1)
		go func() { done <- worldAuth434(left, "ACCOUNT", key434(), 1); left.Close() }()
		if err := write434(right, banner); err != nil {
			t.Error(err)
		}
		if err := <-done; err == nil {
			t.Error("invalid initializer accepted")
		}
		right.Close()
	}
}
