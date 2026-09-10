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
	"net"
	"reflect"
	"testing"
	"time"
)

// Synthetic server writer transcribed from TCPP EnumCharactersResult::Write.
// No production parsing/encoding helper is used to construct these fixtures.
type rosterFixture434 struct {
	bytes.Buffer
	bitByte byte
	used    int
}

func (w *rosterFixture434) bits(v uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		w.bitByte |= byte(v>>i&1) << uint(7-w.used)
		w.used++
		if w.used == 8 {
			w.WriteByte(w.bitByte)
			w.used = 0
			w.bitByte = 0
		}
	}
}
func (w *rosterFixture434) flush() {
	if w.used != 0 {
		w.WriteByte(w.bitByte)
		w.used = 0
		w.bitByte = 0
	}
}
func (w *rosterFixture434) value(v any) {
	if err := binary.Write(&w.Buffer, binary.LittleEndian, v); err != nil {
		panic(err)
	}
}
func (w *rosterFixture434) seq(g uint64, i uint) {
	b := byte(g >> (8 * i))
	if b != 0 {
		w.WriteByte(b ^ 1)
	}
}
func (w *rosterFixture434) mask(g uint64, i uint) {
	v := uint64(0)
	if byte(g>>(8*i)) != 0 {
		v = 1
	}
	w.bits(v, 1)
}

func rosterPacket434(chars []Character434, rules []FactionRestriction434) []byte {
	w := &rosterFixture434{}
	w.bits(uint64(len(rules)), 23)
	w.bits(1, 1)
	w.bits(uint64(len(chars)), 17)
	for _, c := range chars {
		w.mask(c.GUID, 3)
		w.mask(c.GuildGUID, 1)
		w.mask(c.GuildGUID, 7)
		w.mask(c.GuildGUID, 2)
		w.bits(uint64(len(c.Name)), 7)
		w.mask(c.GUID, 4)
		w.mask(c.GUID, 7)
		w.mask(c.GuildGUID, 3)
		w.mask(c.GUID, 5)
		w.mask(c.GuildGUID, 6)
		w.mask(c.GUID, 1)
		w.mask(c.GuildGUID, 5)
		w.mask(c.GuildGUID, 4)
		if c.FirstLogin {
			w.bits(1, 1)
		} else {
			w.bits(0, 1)
		}
		w.mask(c.GUID, 0)
		w.mask(c.GUID, 2)
		w.mask(c.GUID, 6)
		w.mask(c.GuildGUID, 0)
	}
	w.flush()
	for _, c := range chars {
		w.value(c.Class)
		for _, v := range c.Equipment {
			w.value(v.InventoryType)
			w.value(v.DisplayID)
			w.value(v.EnchantmentID)
		}
		w.value(c.PetFamily)
		w.seq(c.GuildGUID, 2)
		w.value(c.ListPosition)
		w.value(c.HairStyle)
		w.seq(c.GuildGUID, 3)
		w.value(c.PetDisplayID)
		w.value(c.Flags)
		w.value(c.HairColor)
		w.seq(c.GUID, 4)
		w.value(c.Map)
		w.seq(c.GuildGUID, 5)
		w.value(c.Z)
		w.seq(c.GuildGUID, 6)
		w.value(c.PetLevel)
		w.seq(c.GUID, 3)
		w.value(c.Y)
		w.value(c.CustomizationFlags)
		w.value(c.FacialHair)
		w.seq(c.GUID, 7)
		w.value(c.Gender)
		w.WriteString(c.Name)
		w.value(c.Face)
		w.seq(c.GUID, 0)
		w.seq(c.GUID, 2)
		w.seq(c.GuildGUID, 1)
		w.seq(c.GuildGUID, 7)
		w.value(c.X)
		w.value(c.Skin)
		w.value(c.Race)
		w.value(c.Level)
		w.seq(c.GUID, 6)
		w.seq(c.GuildGUID, 4)
		w.seq(c.GuildGUID, 0)
		w.seq(c.GUID, 5)
		w.seq(c.GUID, 1)
		w.value(c.Zone)
	}
	for _, r := range rules {
		w.value(r.Mask)
		w.value(r.Race)
	}
	return w.Bytes()
}

func syntheticCharacter434() Character434 {
	c := Character434{GUID: 0x0807060504030201, GuildGUID: 0x1112131415161718, Name: "Tést", Race: 22, Class: 3, Gender: 1, Level: 85, Zone: 1519, Map: 0, X: 1.25, Y: -2.5, Z: 3.75, Skin: 4, Face: 5, HairStyle: 6, HairColor: 7, FacialHair: 8, ListPosition: 2, Flags: 0x10203040, CustomizationFlags: 0x11223344, FirstLogin: true, PetDisplayID: 12345, PetFamily: 23, PetLevel: 84}
	for i := range c.Equipment {
		c.Equipment[i] = CharacterVisual434{uint8(i + 1), uint32(1000 + i), uint32(2000 + i)}
	}
	return c
}

func TestRoster434Fixtures(t *testing.T) {
	// Fixed build-15595 empty roster fixture (41 bits padded to six bytes).
	empty, _ := hex.DecodeString("000001000000")
	r, err := parseRoster434(empty)
	if err != nil || len(r.Characters) != 0 || len(r.Restrictions) != 0 {
		t.Fatalf("empty: %+v %v", r, err)
	}
	a := syntheticCharacter434()
	b := a
	b.Name = "Sparse"
	b.GUID = 0x0100000001000001
	b.GuildGUID = 0
	b.FirstLogin = false
	b.Map = 530
	for _, chars := range [][]Character434{{a}, {a, b}, {b, a, b, a}} {
		rules := []FactionRestriction434{{Mask: -123456, Race: 22}, {Mask: 0x12345678, Race: 1}}
		got, err := parseRoster434(rosterPacket434(chars, rules))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Characters, chars) || !reflect.DeepEqual(got.Restrictions, rules) {
			t.Fatalf("roster mismatch\ngot %+v\nwant %+v", got, chars)
		}
	}
}

func TestRoster434Malformed(t *testing.T) {
	b := rosterPacket434([]Character434{syntheticCharacter434()}, nil)
	badNameLength := append([]byte(nil), b...)
	// Name length occupies bits 45..51 in the first character's metadata.
	for bit := 45; bit < 52; bit++ {
		badNameLength[bit/8] |= 1 << uint(7-bit%8)
	}
	if _, err := parseRoster434(badNameLength); err == nil {
		t.Fatal("accepted name length exceeding available body")
	}
	for n := 0; n < len(b); n++ {
		if _, err := parseRoster434(b[:n]); err == nil {
			t.Fatalf("accepted truncation at %d", n)
		}
	}
	for _, bad := range [][]byte{append(append([]byte{}, b...), 0), {0xff, 0xff, 0xff, 0xff, 0xff, 0x80}, {0, 0, 0, 0, 0, 0}} {
		if _, err := parseRoster434(bad); err == nil {
			t.Fatalf("accepted malformed packet")
		}
	}
	c := syntheticCharacter434()
	c.Name = ""
	if _, err := parseRoster434(rosterPacket434([]Character434{c}, nil)); err == nil {
		t.Fatal("accepted zero name length")
	}
	c.Name = "A\x00B"
	if _, err := parseRoster434(rosterPacket434([]Character434{c}, nil)); err == nil {
		t.Fatal("accepted NUL name")
	}
	c.Name = "\xff"
	if _, err := parseRoster434(rosterPacket434([]Character434{c}, nil)); err == nil {
		t.Fatal("accepted invalid UTF8")
	}
}

func TestCharacterEnum434Transport(t *testing.T) {
	for _, n := range []int{0, 1, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			chars := make([]Character434, n)
			for i := range chars {
				chars[i] = syntheticCharacter434()
				chars[i].GUID += uint64(i)
				chars[i].Name = fmt.Sprintf("Test%d", i)
			}
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			done := make(chan error, 1)
			go func() {
				done <- mockWorldThen434(ln, func(conn net.Conn, send *rc4.Cipher) error {
					var header [6]byte
					if _, err := io.ReadFull(conn, header[:]); err != nil {
						return err
					}
					seed, _ := hex.DecodeString("c2b3723cc6aed9b5343c53ee2f4367ce")
					h := hmac.New(sha1.New, seed)
					h.Write(key434())
					recv, _ := rc4.NewCipher(h.Sum(nil))
					drop := make([]byte, 1024)
					recv.XORKeyStream(drop, drop)
					recv.XORKeyStream(header[:], header[:])
					if header != [6]byte{0, 4, 2, 5, 0, 0} {
						return fmt.Errorf("unexpected request/header: %x", header)
					}
					body := rosterPacket434(chars, nil)
					var wire []byte
					if n == 4 {
						var compressed bytes.Buffer
						z := zlib.NewWriter(&compressed)
						previous := 0
						for _, packet := range []struct {
							op uint16
							b  []byte
						}{{0x1234, bytes.Repeat([]byte{42}, 1200)}, {0x10b0, body}} {
							z.Write(packet.b)
							z.Flush()
							chunk := binary.LittleEndian.AppendUint32(nil, uint32(len(packet.b)))
							chunk = append(chunk, compressed.Bytes()[previous:]...)
							previous = compressed.Len()
							wire = append(wire, frame434(packet.op|0x8000, chunk, send)...)
						}
						z.Close()
					} else {
						wire = frame434(0x10b0, body, send)
					}
					for _, b := range wire {
						if err := write434(conn, []byte{b}); err != nil {
							return err
						}
					}
					return nil
				})
			}()
			got, err := EnumerateCharacters434(context.Background(), "account", key434(), RealmInfo{ID: 73, Address: ln.Addr().String()})
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(got.Characters, chars) {
				t.Errorf("transport roster mismatch: %+v", got)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(6 * time.Second):
				t.Fatal("server did not observe close")
			}
		})
	}
}

func TestCompressed434BadLength(t *testing.T) {
	var compressed bytes.Buffer
	z := zlib.NewWriter(&compressed)
	z.Write([]byte{1, 2, 3})
	z.Flush()
	defer z.Close()
	for _, size := range []uint32{0, 2, 4, 0xffffffff} {
		left, right := net.Pipe()
		left.SetDeadline(time.Now().Add(time.Second))
		right.SetDeadline(time.Now().Add(time.Second))
		body := binary.LittleEndian.AppendUint32(nil, size)
		body = append(body, compressed.Bytes()...)
		go func() { write434(right, frame434(0x90b0, body, nil)); right.Close() }()
		w := worldWire434{conn: left}
		if _, _, err := w.read(); err == nil {
			t.Errorf("accepted advertised length %d", size)
		}
		left.Close()
	}
}
