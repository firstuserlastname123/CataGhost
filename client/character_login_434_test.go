package client

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/hmac"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func redirectFixture434(t *testing.T) []byte {
	t.Helper()
	s, e := os.ReadFile("testdata/connect_to_434_synthetic.hex")
	if e != nil {
		t.Fatal(e)
	}
	b, e := hex.DecodeString(strings.TrimSpace(string(s)))
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestLogin434GUIDFixture(t *testing.T) {
	for _, tc := range []struct {
		g   uint64
		hex string
	}{{2, "2003"}, {0x0807060504030201, "ff0209000507060304"}, {0x0100000001000001, "61000000"}} {
		got := hex.EncodeToString(loginGUID434(tc.g))
		if got != tc.hex {
			t.Fatalf("guid %x got %s want %s", tc.g, got, tc.hex)
		}
	}
}
func TestLogin434Selection(t *testing.T) {
	c := Character434{Name: "Ghost", GUID: 2}
	if got, e := selectLoginCharacter434(CharacterRoster434{Characters: []Character434{c}}, "Ghost"); e != nil || got.GUID != 2 {
		t.Fatal(got, e)
	}
	for _, r := range []CharacterRoster434{{}, {Characters: []Character434{c}}, {Characters: []Character434{c, c}}} {
		if _, e := selectLoginCharacter434(r, "Missing"); e == nil {
			t.Fatal("accepted missing character")
		}
	}
	if _, e := selectLoginCharacter434(CharacterRoster434{Characters: []Character434{c, c}}, "Ghost"); e == nil {
		t.Fatal("accepted duplicate")
	}
}
func TestLogin434RedirectFixture(t *testing.T) {
	b := redirectFixture434(t)
	key, address, e := redirect434(b)
	if e != nil || key != 0x10000007b || address != "127.0.0.1:8087" {
		t.Fatal(key, address, e)
	}
	for n := 0; n < len(b); n++ {
		if _, _, e := redirect434(b[:n]); e == nil {
			t.Fatalf("accepted truncation %d", n)
		}
	}
	b[44] ^= 1
	if _, _, e := redirect434(b); e == nil {
		t.Fatal("accepted corrupt signed payload")
	}
}
func TestLogin434Verification(t *testing.T) {
	b := binary.LittleEndian.AppendUint32(nil, 1)
	for _, f := range []float32{1.5, 2.5, 3.5, 0.75} {
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(f))
	}
	var r CharacterLogin434Result
	if e := verifyWorld434(b, &r); e != nil || r.Map != 1 || r.X != 1.5 || r.Orientation != 0.75 {
		t.Fatal(r, e)
	}
	for n := 0; n < 20; n++ {
		if verifyWorld434(b[:n], &r) == nil {
			t.Fatal("accepted truncation")
		}
	}
	binary.LittleEndian.PutUint32(b[4:], 0x7fc00000)
	if verifyWorld434(b, &r) == nil {
		t.Fatal("accepted NaN")
	}
}

func independentCrypt434(key, seed []byte) *rc4.Cipher {
	h := hmac.New(sha1.New, seed)
	h.Write(key)
	c, _ := rc4.NewCipher(h.Sum(nil))
	drop := make([]byte, 1024)
	c.XORKeyStream(drop, drop)
	return c
}
func readClient434(conn net.Conn, c *rc4.Cipher) (uint32, []byte, error) {
	var h [6]byte
	if _, e := io.ReadFull(conn, h[:]); e != nil {
		return 0, nil, e
	}
	if c != nil {
		c.XORKeyStream(h[:], h[:])
	}
	size := int(binary.BigEndian.Uint16(h[:2])) - 4
	if size < 0 || size > 10236 {
		return 0, nil, fmt.Errorf("invalid client size")
	}
	b := make([]byte, size)
	_, e := io.ReadFull(conn, b)
	return binary.LittleEndian.Uint32(h[2:]), b, e
}

func TestLogin434SyntheticTwoSockets(t *testing.T) {
	for _, tc := range []struct{ compressed, observe bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			compressed := tc.compressed
			fixture := redirectFixture434(t)
			realmLn, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			defer realmLn.Close()
			instanceLn, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			defer instanceLn.Close()
			doneRealm := make(chan error, 1)
			doneInstance := make(chan error, 1)
			var updates [][]byte
			if tc.observe {
				updates = append(updates, updateFixture434(livingFixture434(2, 4, true), livingFixture434(0xf130000100000003, 3, true)))
			}
			go func() { doneInstance <- mockLoginInstance434(instanceLn, compressed, updates...) }()
			go func() {
				doneRealm <- mockWorldThen434(realmLn, func(conn net.Conn, send *rc4.Cipher) error {
					seed, _ := hex.DecodeString("c2b3723cc6aed9b5343c53ee2f4367ce")
					recv := independentCrypt434(key434(), seed)
					op, b, e := readClient434(conn, recv)
					if e != nil || op != 0x0502 || len(b) != 0 {
						return fmt.Errorf("enum request %x: %v", op, e)
					}
					c := syntheticCharacter434()
					c.Name = "Ghost"
					c.GUID = 2
					if e := write434(conn, frame434(0x10b0, rosterPacket434([]Character434{c}, nil), send)); e != nil {
						return e
					}
					op, b, e = readClient434(conn, recv)
					if e != nil || op != 0x05b1 || !bytes.Equal(b, []byte{0x20, 3}) {
						return fmt.Errorf("login GUID/header continuity %x %x: %v", op, b, e)
					}
					return write434(conn, frame434(0x0942, fixture, send))
				})
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var result CharacterLogin434Result
			err := withWorld434(ctx, "ACCOUNT", key434(), RealmInfo{ID: 73, Address: realmLn.Addr().String()}, func(w *worldWire434) error {
				r, e := enumerateOnWorld434(w)
				if e != nil {
					return e
				}
				result.Character, e = selectLoginCharacter434(r, "Ghost")
				if e != nil {
					return e
				}
				if e := w.send(cataPlayerLogin, loginGUID434(result.Character.GUID)); e != nil {
					return e
				}
				connect := func(ctx context.Context, address, user string, key []byte, link uint64) (*worldWire434, error) {
					if address != "127.0.0.1:8087" {
						return nil, fmt.Errorf("wrong redirect")
					}
					// Only the synthetic test redirects dialing to an ephemeral listener.
					return openInstance434(ctx, instanceLn.Addr().String(), user, key, link)
				}
				if !tc.observe {
					return awaitLoginWith434(ctx, w, "ACCOUNT", "127.0.0.1:8087", &result, connect)
				}
				s := WorldState434Result{Store: ObjectStore434{PlayerGUID: result.Character.GUID}, Opcodes: make(map[uint16]int), WorldVariables: make(map[uint32]int32)}
				err := awaitObservedLogin434(ctx, w, "ACCOUNT", "127.0.0.1:8087", &result, connect, s.observe, func() (bool, error) { s.Login = result; return s.playerReady() })
				if err == nil && (len(s.Store.Objects()) != 2 || s.Opcodes[cataUpdateObject] != 1) {
					return fmt.Errorf("initial updates were lost")
				}
				return err
			})
			if err != nil {
				t.Error(err)
			} else if result.Character.GUID != 2 || result.Map != 1 || result.TimeSyncCounter != 7 {
				t.Error(result)
			}
			for _, ch := range []chan error{doneRealm, doneInstance} {
				select {
				case err := <-ch:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(6 * time.Second):
					t.Fatal("server did not observe clean close")
				}
			}
		})
	}
}

func mockLoginInstance434(ln net.Listener, compressed bool, updates ...[]byte) error {
	conn, e := ln.Accept()
	if e != nil {
		return e
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if e := write434(conn, append([]byte{0, byte(len(cataServerBanner))}, cataServerBanner...)); e != nil {
		return e
	}
	banner := make([]byte, 2+len(cataClientBanner))
	if _, e := io.ReadFull(conn, banner); e != nil {
		return e
	}
	if string(banner[2:]) != cataClientBanner {
		return fmt.Errorf("instance banner mismatch")
	}
	challenge := make([]byte, 37)
	for i := 0; i < 32; i++ {
		challenge[i] = byte(i + 1)
	}
	copy(challenge[32:], []byte{9, 10, 11, 12, 1})
	if e := write434(conn, frame434(0x4542, challenge, nil)); e != nil {
		return e
	}
	op, b, e := readClient434(conn, nil)
	if e != nil || op != 0x044d || len(b) != 36 {
		return fmt.Errorf("continued session %x size %d: %v", op, len(b), e)
	}
	if binary.LittleEndian.Uint64(b) != 0x10000007b {
		return fmt.Errorf("continued-session link mismatch")
	}
	h := sha1.New()
	h.Write([]byte("ACCOUNT"))
	h.Write(key434())
	h.Write([]byte{9, 10, 11, 12})
	digest := h.Sum(nil)
	order := []int{5, 2, 6, 10, 8, 17, 11, 15, 7, 1, 4, 16, 0, 12, 14, 13, 18, 9, 19, 3}
	for i, index := range order {
		if b[16+i] != digest[index] {
			return fmt.Errorf("continued-session digest mismatch")
		}
	}
	send := independentCrypt434(key434(), challenge[:16])
	recv := independentCrypt434(key434(), challenge[16:32])
	verify := binary.LittleEndian.AppendUint32(nil, 1)
	for _, f := range []float32{1.25, -2.5, 3.75, 0.5} {
		verify = binary.LittleEndian.AppendUint32(verify, math.Float32bits(f))
	}
	wire := frame434(0x0140, nil, send)
	var zbuf bytes.Buffer
	z := zlib.NewWriter(&zbuf)
	defer z.Close()
	appendPacket := func(op uint16, body []byte) {
		if compressed {
			start := zbuf.Len()
			z.Write(body)
			z.Flush()
			payload := binary.LittleEndian.AppendUint32(nil, uint32(len(body)))
			payload = append(payload, zbuf.Bytes()[start:]...)
			wire = append(wire, frame434(op|0x8000, payload, send)...)
		} else {
			wire = append(wire, frame434(op, body, send)...)
		}
	}
	// Initial object updates intentionally precede LOGIN_VERIFY_WORLD/time sync.
	for _, body := range updates {
		appendPacket(0x4715, body)
	}
	appendPacket(0x2005, verify)
	wire = append(wire, frame434(0x3ca4, []byte{7, 0, 0, 0}, send)...)
	for _, x := range wire {
		if e := write434(conn, []byte{x}); e != nil {
			return e
		}
	}
	op, b, e = readClient434(conn, recv)
	if e != nil || op != 0x3b0c || len(b) != 8 || binary.LittleEndian.Uint32(b) != 7 {
		return fmt.Errorf("required time sync ack missing: %x %v", op, e)
	}
	var extra [1]byte
	n, e := conn.Read(extra[:])
	if n != 0 || e != io.EOF {
		return fmt.Errorf("expected EOF with no gameplay, got %d: %v", n, e)
	}
	return nil
}

func TestLogin434RejectAndCancel(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   []byte
		cancel bool
	}{{"rejected", []byte{0}, false}, {"malformed", nil, false}, {"cancel", nil, true}} {
		t.Run(tc.name, func(t *testing.T) {
			left, right := net.Pipe()
			defer right.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if tc.cancel {
				cancel()
			} else {
				go func() { write434(right, frame434(cataCharacterLoginFailed, tc.body, nil)) }()
			}
			var result CharacterLogin434Result
			if err := awaitLogin434(ctx, &worldWire434{conn: left}, "ACCOUNT", "127.0.0.1:8087", &result); err == nil {
				t.Fatal("failure reported success")
			}
		})
	}
}
