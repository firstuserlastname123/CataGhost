package client

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/hmac"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"unicode/utf8"
)

const cataEnumCharacters = 0x0502
const cataEnumCharactersResult = 0x10B0

type CharacterVisual434 struct {
	InventoryType            uint8
	DisplayID, EnchantmentID uint32
}
type Character434 struct {
	GUID, GuildGUID                                            uint64
	Name                                                       string
	Race, Class, Gender, Level                                 uint8
	Zone, Map                                                  int32
	X, Y, Z                                                    float32
	Skin, Face, HairStyle, HairColor, FacialHair, ListPosition uint8
	Flags, CustomizationFlags                                  uint32
	FirstLogin                                                 bool
	PetDisplayID, PetFamily, PetLevel                          uint32
	Equipment                                                  [23]CharacterVisual434
}
type FactionRestriction434 struct {
	Mask int32
	Race uint8
}
type CharacterRoster434 struct {
	Characters   []Character434
	Restrictions []FactionRestriction434
}

// EnumerateCharacters434 authenticates, requests the roster once, and closes.
// It does not select, create or log in any character.
func EnumerateCharacters434(ctx context.Context, username string, key []byte, realm RealmInfo) (CharacterRoster434, error) {
	var roster CharacterRoster434
	err := withWorld434(ctx, username, key, realm, func(w *worldWire434) error {
		var err error
		roster, err = enumerateOnWorld434(w)
		return err
	})
	return roster, err
}

func enumerateOnWorld434(w *worldWire434) (CharacterRoster434, error) {
	if err := w.send(cataEnumCharacters, nil); err != nil {
		return CharacterRoster434{}, fmt.Errorf("character enumeration request: %w", err)
	}
	for i := 0; i < 64; i++ {
		op, data, err := w.read()
		if err != nil {
			return CharacterRoster434{}, fmt.Errorf("character enumeration response: %w", err)
		}
		if op == cataEnumCharactersResult {
			return parseRoster434(data)
		}
	}
	return CharacterRoster434{}, fmt.Errorf("character enumeration response missing after 64 packets")
}

func packetCrypt434(key, seed []byte) *rc4.Cipher {
	h := hmac.New(sha1.New, seed)
	h.Write(key)
	c, _ := rc4.NewCipher(h.Sum(nil))
	drop := make([]byte, 1024)
	c.XORKeyStream(drop, drop)
	return c
}

func (w *worldWire434) send(op uint16, body []byte) error {
	if len(body)+4 >= 10240 {
		return fmt.Errorf("outbound packet exceeds server limit")
	}
	if w.transmit == nil {
		w.transmit = packetCrypt434(w.key, []byte{0xc2, 0xb3, 0x72, 0x3c, 0xc6, 0xae, 0xd9, 0xb5, 0x34, 0x3c, 0x53, 0xee, 0x2f, 0x43, 0x67, 0xce})
	}
	header := binary.BigEndian.AppendUint16(nil, uint16(len(body)+4))
	header = binary.LittleEndian.AppendUint32(header, uint32(op))
	w.transmit.XORKeyStream(header, header)
	return write434(w.conn, append(header, body...))
}

type worldWire434 struct {
	transmit   *rc4.Cipher
	conn       net.Conn
	receive    *rc4.Cipher
	key        []byte
	compressed []byte
	decoded    int
}

func (w *worldWire434) read() (uint16, []byte, error) {
	op, b, err := read434(w.conn, w.receive)
	if err != nil || op&0x8000 == 0 {
		return op, b, err
	}
	if len(b) < 4 {
		return 0, nil, fmt.Errorf("truncated compressed packet")
	}
	n := int(binary.LittleEndian.Uint32(b[:4]))
	if n < 1 || n > 1<<20 || w.decoded+n > 4<<20 || len(w.compressed)+len(b)-4 > 4<<20 {
		return 0, nil, fmt.Errorf("compressed packet exceeds probe limits")
	}
	w.compressed = append(w.compressed, b[4:]...)
	z, err := zlib.NewReader(bytes.NewReader(w.compressed))
	if err != nil {
		return 0, nil, fmt.Errorf("zlib header: %w", err)
	}
	defer z.Close()
	// A Z_SYNC_FLUSH stream has no final checksum yet. UnexpectedEOF is expected,
	// but the exact cumulative output must match all declared packet lengths.
	data, err := io.ReadAll(io.LimitReader(z, int64(w.decoded+n+1)))
	if err != nil && err != io.ErrUnexpectedEOF {
		return 0, nil, fmt.Errorf("zlib stream: %w", err)
	}
	if len(data) != w.decoded+n {
		return 0, nil, fmt.Errorf("compressed packet length mismatch")
	}
	result := data[w.decoded:]
	w.decoded += n
	return op &^ 0x8000, result, nil
}

type rosterReader434 struct {
	b   []byte
	bit int
	err error
}

func (r *rosterReader434) bits(n int) uint32 {
	if r.err != nil {
		return 0
	}
	if r.bit+n > len(r.b)*8 {
		r.err = io.ErrUnexpectedEOF
		return 0
	}
	var v uint32
	for i := 0; i < n; i++ {
		v = v<<1 | uint32((r.b[r.bit/8]>>uint(7-r.bit%8))&1)
		r.bit++
	}
	return v
}
func (r *rosterReader434) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	i := r.bit / 8
	if n < 0 || i+n > len(r.b) {
		r.err = io.ErrUnexpectedEOF
		return nil
	}
	r.bit += n * 8
	return r.b[i : i+n]
}
func (r *rosterReader434) u8() uint8 {
	b := r.take(1)
	if b == nil {
		return 0
	}
	return b[0]
}
func (r *rosterReader434) u32() uint32 {
	b := r.take(4)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(b)
}
func (r *rosterReader434) f32() float32 { return math.Float32frombits(r.u32()) }
func (r *rosterReader434) guid(g *[8]byte, i int) {
	if g[i] != 0 {
		g[i] = r.u8() ^ 1
	}
}

// Inverse of TCPP CharacterPackets.cpp EnumCharactersResult::Write.
func parseRoster434(b []byte) (CharacterRoster434, error) {
	fail := func(err error) (CharacterRoster434, error) {
		return CharacterRoster434{}, fmt.Errorf("character roster: %w", err)
	}
	r := &rosterReader434{b: b}
	restrictions := int(r.bits(23))
	success := r.bits(1) != 0
	count := int(r.bits(17))
	if r.err != nil {
		return fail(r.err)
	}
	if !success {
		return fail(fmt.Errorf("server reported unsuccessful enumeration"))
	}
	// Each character needs 24 mask bits plus at least 257 body bytes.
	if count > len(b)/260 || restrictions > len(b)/5 || (41+count*24+7)/8+count*257+restrictions*5 > len(b) {
		return fail(fmt.Errorf("counts exceed packet length"))
	}
	roster := CharacterRoster434{Characters: make([]Character434, count), Restrictions: make([]FactionRestriction434, restrictions)}
	type mask struct {
		g, h [8]byte
		name int
	}
	masks := make([]mask, count)
	for i := range masks {
		m := &masks[i]
		c := &roster.Characters[i]
		m.g[3] = byte(r.bits(1))
		m.h[1] = byte(r.bits(1))
		m.h[7] = byte(r.bits(1))
		m.h[2] = byte(r.bits(1))
		m.name = int(r.bits(7))
		m.g[4] = byte(r.bits(1))
		m.g[7] = byte(r.bits(1))
		m.h[3] = byte(r.bits(1))
		m.g[5] = byte(r.bits(1))
		m.h[6] = byte(r.bits(1))
		m.g[1] = byte(r.bits(1))
		m.h[5] = byte(r.bits(1))
		m.h[4] = byte(r.bits(1))
		c.FirstLogin = r.bits(1) != 0
		m.g[0] = byte(r.bits(1))
		m.g[2] = byte(r.bits(1))
		m.g[6] = byte(r.bits(1))
		m.h[0] = byte(r.bits(1))
	}
	r.bit = (r.bit + 7) / 8 * 8
	for i := range roster.Characters {
		c := &roster.Characters[i]
		m := &masks[i]
		c.Class = r.u8()
		for j := range c.Equipment {
			c.Equipment[j] = CharacterVisual434{r.u8(), r.u32(), r.u32()}
		}
		c.PetFamily = r.u32()
		r.guid(&m.h, 2)
		c.ListPosition = r.u8()
		c.HairStyle = r.u8()
		r.guid(&m.h, 3)
		c.PetDisplayID = r.u32()
		c.Flags = r.u32()
		c.HairColor = r.u8()
		r.guid(&m.g, 4)
		c.Map = int32(r.u32())
		r.guid(&m.h, 5)
		c.Z = r.f32()
		r.guid(&m.h, 6)
		c.PetLevel = r.u32()
		r.guid(&m.g, 3)
		c.Y = r.f32()
		c.CustomizationFlags = r.u32()
		c.FacialHair = r.u8()
		r.guid(&m.g, 7)
		c.Gender = r.u8()
		c.Name = string(r.take(m.name))
		c.Face = r.u8()
		r.guid(&m.g, 0)
		r.guid(&m.g, 2)
		r.guid(&m.h, 1)
		r.guid(&m.h, 7)
		c.X = r.f32()
		c.Skin = r.u8()
		c.Race = r.u8()
		c.Level = r.u8()
		r.guid(&m.g, 6)
		r.guid(&m.h, 4)
		r.guid(&m.h, 0)
		r.guid(&m.g, 5)
		r.guid(&m.g, 1)
		c.Zone = int32(r.u32())
		if r.err != nil {
			return fail(r.err)
		}
		if c.Name == "" || !utf8.ValidString(c.Name) || bytes.IndexByte([]byte(c.Name), 0) >= 0 {
			return fail(fmt.Errorf("invalid character name"))
		}
		c.GUID = binary.LittleEndian.Uint64(m.g[:])
		c.GuildGUID = binary.LittleEndian.Uint64(m.h[:])
	}
	for i := range roster.Restrictions {
		roster.Restrictions[i] = FactionRestriction434{int32(r.u32()), r.u8()}
	}
	if r.err != nil {
		return fail(r.err)
	}
	if r.bit/8 != len(b) {
		return fail(fmt.Errorf("unexpected trailing bytes"))
	}
	return roster, nil
}
