package client

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"sort"
	"testing"
	"time"
)

// Independent source-derived server fixture writer. Numeric indices are literal
// so a mistaken production constant cannot change the expected wire fixture.
func packedFixture434(g uint64) []byte {
	b := []byte{0}
	for i := uint(0); i < 8; i++ {
		v := byte(g >> (i * 8))
		if v != 0 {
			b[0] |= 1 << i
			b = append(b, v)
		}
	}
	return b
}
func fieldsFixture434(fields map[uint16]uint32) []byte {
	keys := make([]int, 0, len(fields))
	for k := range fields {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	n := 0
	if len(keys) > 0 {
		n = keys[len(keys)-1]/32 + 1
	}
	w := &rosterFixture434{}
	w.value(uint8(n))
	mask := make([]uint32, n)
	for _, k := range keys {
		mask[k/32] |= 1 << uint(k%32)
	}
	for _, m := range mask {
		w.value(m)
	}
	for _, k := range keys {
		w.value(fields[uint16(k)])
	}
	return w.Bytes()
}
func updateFixture434(blocks ...[]byte) []byte {
	w := &rosterFixture434{}
	w.value(uint16(1))
	w.value(uint32(len(blocks)))
	for _, b := range blocks {
		w.Write(b)
	}
	return w.Bytes()
}
func valuesFixture434(g uint64, fields map[uint16]uint32) []byte {
	b := append([]byte{0}, packedFixture434(g)...)
	return append(b, fieldsFixture434(fields)...)
}

// Living create: tests every GUID byte, active/finalized spline variants,
// optional transport/fall/pitch, and nonzero orientation without movement sends.
func livingFixture434(g uint64, typ uint8, rich bool) []byte {
	w := &rosterFixture434{}
	w.value(uint8(2))
	w.Write(packedFixture434(g))
	w.value(typ)
	flags := uint64(1)
	if typ == 4 {
		flags |= 4
	}
	w.bits(flags, 8)
	w.bits(0, 24)
	w.bits(0, 6)
	w.bits(1, 1)
	w.bits(0, 1)
	for _, i := range []uint{7, 3, 2} {
		w.mask(g, i)
	}
	w.bits(0, 1)
	if rich {
		w.bits(0, 1)
		w.bits(1, 1)
		w.bits(1, 1)
		w.bits(0, 1)
	} else {
		w.bits(1, 1)
		w.bits(0, 1)
		w.bits(0, 1)
		w.bits(1, 1)
	}
	w.mask(g, 5)
	if rich {
		w.bits(1, 1)
	} else {
		w.bits(0, 1)
	}
	w.bits(0, 1)
	transport := uint64(0x0807060504030201)
	if rich {
		w.mask(transport, 1)
		w.bits(1, 1)
		for _, i := range []uint{4, 0, 6} {
			w.mask(transport, i)
		}
		w.bits(1, 1)
		for _, i := range []uint{7, 5, 3, 2} {
			w.mask(transport, i)
		}
	}
	w.mask(g, 4)
	if rich {
		w.bits(1, 1)
		w.bits(0, 2)
		w.bits(1, 1)
		w.bits(2, 22)
		w.bits(2, 2)
		for _, i := range []uint{4, 3, 7, 2, 6, 1, 0, 5} {
			w.mask(transport, i)
		}
		w.bits(1, 1)
		w.bits(0, 25)
	}
	w.mask(g, 6)
	if rich {
		w.bits(1, 1)
	}
	w.mask(g, 0)
	w.mask(g, 1)
	w.bits(0, 1)
	w.bits(1, 1)
	w.flush()
	w.seq(g, 4)
	w.value(float32(4.5))
	if rich {
		w.value([3]float32{1, 0, 1})
		w.value(uint32(42))
		w.value(float32(0))
	}
	w.value(float32(2.5))
	if rich {
		w.value(float32(0.25))
		w.value(float32(0))
		w.value(uint32(10))
		for _, i := range []uint{5, 3, 7, 1, 6, 4, 2, 0} {
			w.seq(transport, i)
		}
		w.value([6]float32{3, 1, 2, 6, 4, 5})
		w.value(float32(1))
		w.value(uint32(100))
		w.value(uint32(0))
		w.value(float32(1))
		w.value([3]float32{3, 1, 2})
		w.value(uint32(123))
	}
	w.value(float32(3.75))
	w.seq(g, 5)
	if rich {
		w.seq(transport, 5)
		w.seq(transport, 7)
		w.value(uint32(20))
		w.value(float32(0.5))
		w.value(uint32(21))
		w.value(float32(2))
		w.value(float32(1))
		w.seq(transport, 3)
		w.value(float32(3))
		w.seq(transport, 0)
		w.value(uint32(22))
		w.value(int8(-1))
		for _, i := range []uint{1, 6, 2, 4} {
			w.seq(transport, i)
		}
	}
	w.value(float32(1.25))
	w.value(float32(3.14))
	w.seq(g, 3)
	w.seq(g, 0)
	w.value(float32(4.7))
	w.value(float32(-2.5))
	for _, i := range []uint{7, 1, 2} {
		w.seq(g, i)
	}
	w.value(float32(2.5))
	w.value(uint32(12345))
	w.value(float32(3.14))
	w.seq(g, 6)
	w.value(float32(7))
	w.value(float32(0.5))
	w.value(float32(7))
	if rich {
		w.value(float32(0.1))
	}
	w.value(float32(4.5))
	w.Write(fieldsFixture434(map[uint16]uint32{0: uint32(g), 1: uint32(g >> 32), 0x19: 0x02000304, 0x1a: 125, 0x1b: 100, 0x20: 150, 0x21: 100, 0x30: 7, 0x31: 35, 0x3d: 123, 0x567: 0x12345678}))
	return w.Bytes()
}

func TestObject434CreateAndMasks(t *testing.T) {
	for _, rich := range []bool{false, true} {
		t.Run(fmt.Sprint(rich), func(t *testing.T) {
			g := uint64(0x0807060504030201)
			s := ObjectStore434{PlayerGUID: g}
			if err := s.ApplyUpdate(updateFixture434(livingFixture434(g, 4, rich), livingFixture434(0xf130000100000002, 3, rich))); err != nil {
				t.Fatal(err)
			}
			objects := s.Objects()
			if len(objects) != 2 {
				t.Fatal(objects)
			}
			p := s.objects[g]
			if !p.Created || !p.ThisIsYou || p.Type != 4 || p.Fields[0x30] != 7 || p.Fields[0x567] != 0x12345678 || *p.Position != (Position434{1.25, -2.5, 3.75, 0.5}) {
				t.Fatal(p)
			}
			if err := s.ApplyUpdate(updateFixture434(valuesFixture434(g, map[uint16]uint32{0x1a: 80, 0x1b: 0}))); err != nil {
				t.Fatal(err)
			}
			if p.Fields[0x1a] != 80 || p.Fields[0x1b] != 0 || p.Fields[0x30] != 7 || p.Revision != 3 {
				t.Fatal(p)
			}
			objects[0].Fields[0x30] = 999
			objects[0].Position.X = 999
			if p.Fields[0x30] != 7 || p.Position.X != 1.25 {
				t.Fatal("snapshot mutated store")
			}
		})
	}
}
func TestObject434LifetimeAtomicity(t *testing.T) {
	s := ObjectStore434{PlayerGUID: 17}
	b := updateFixture434(livingFixture434(17, 4, false))
	if err := s.ApplyUpdate(b); err != nil {
		t.Fatal(err)
	}
	before := s.Objects()
	for i := 0; i < len(b); i++ {
		if err := s.ApplyUpdate(b[:i]); err == nil {
			t.Fatalf("accepted prefix %d", i)
		}
		if !reflect.DeepEqual(before, s.Objects()) {
			t.Fatal("partial mutation")
		}
	}
	for _, bad := range [][]byte{append(append([]byte(nil), b...), 0), {1, 0, 255, 255, 255, 255}, {1, 0, 1, 0, 0, 0, 9}, {1, 0, 1, 0, 0, 0, 3, 255, 255, 255, 255}, {1, 0, 1, 0, 0, 0, 0, 1, 17, 255}} {
		if s.ApplyUpdate(bad) == nil {
			t.Fatalf("accepted %x", bad)
		}
	}
	remove := append([]byte{3, 1, 0, 0, 0}, packedFixture434(17)...)
	if err := s.ApplyUpdate(updateFixture434(remove)); err != nil {
		t.Fatal(err)
	}
	if len(s.Objects()) != 0 || s.PlayerGUID != 17 {
		t.Fatal("removal lost identity")
	}
	if err := s.ApplyUpdate(updateFixture434(valuesFixture434(17, map[uint16]uint32{0x30: 8}))); err != nil {
		t.Fatal(err)
	}
	if s.objects[17].Created {
		t.Fatal("values invented creation")
	}
	if err := s.ApplyUpdate(b); err != nil {
		t.Fatal(err)
	}
	if s.objects[17].Fields[0x30] != 7 {
		t.Fatal("recreate retained old values")
	}
	if s.ApplyDestroy([]byte{17}) == nil {
		t.Fatal("truncated destroy")
	}
	destroy := binary.LittleEndian.AppendUint64(nil, 17)
	destroy = append(destroy, 1)
	if err := s.ApplyDestroy(destroy); err != nil {
		t.Fatal(err)
	}
	if len(s.Objects()) != 0 {
		t.Fatal("destroy failed")
	}
	if err := s.ApplyUpdate(b); err != nil {
		t.Fatal(err)
	}
	s.setMap(2)
	if len(s.Objects()) != 0 || s.PlayerGUID != 17 {
		t.Fatal("map change retained stale objects")
	}
	if s.ApplyUpdate(updateFixture434(livingFixture434(18, 4, false))) == nil {
		t.Fatal("wrong self identity")
	}
}
func TestObject434PackedGUID(t *testing.T) {
	for _, g := range []uint64{0, 1, 0x100, 0xff00000000000001, 0x0807060504030201} {
		r := newObjectReader434(packedFixture434(g))
		if got := r.packed(); got != g || r.done() != nil {
			t.Fatalf("GUID %x got %x", g, got)
		}
	}
	r := newObjectReader434([]byte{255, 1})
	r.packed()
	if r.err == nil {
		t.Fatal("truncated packed GUID")
	}
}
func TestObject434CompressedStream(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	left.SetDeadline(time.Now().Add(3 * time.Second))
	bodies := [][]byte{updateFixture434(livingFixture434(17, 4, false)), updateFixture434(valuesFixture434(17, map[uint16]uint32{0x1a: 99}))}
	var zbuf bytes.Buffer
	z := zlib.NewWriter(&zbuf)
	var wire []byte
	for _, body := range bodies {
		start := zbuf.Len()
		z.Write(body)
		z.Flush()
		payload := binary.LittleEndian.AppendUint32(nil, uint32(len(body)))
		payload = append(payload, zbuf.Bytes()[start:]...)
		wire = append(wire, frame434(0xc715, payload, nil)...)
	}
	z.Close()
	done := make(chan error, 1)
	go func() {
		for _, b := range wire {
			if err := write434(right, []byte{b}); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	w := worldWire434{conn: left}
	s := ObjectStore434{PlayerGUID: 17}
	for range bodies {
		op, b, err := w.read()
		if err != nil || op != 0x4715 {
			t.Fatalf("%x %v", op, err)
		}
		if err := s.ApplyUpdate(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if s.objects[17].Fields[0x1a] != 99 {
		t.Fatal("compressed continuity")
	}
}
func TestObject434PrimaryPower(t *testing.T) {
	for _, tc := range []struct {
		class, power uint8
		slot         uint16
	}{{3, 2, 0}, {11, 3, 2}, {2, 9, 1}, {9, 7, 1}} {
		got, ok := PrimaryPowerSlot434(tc.class, tc.power)
		if !ok || got != tc.slot {
			t.Fatal(tc, got)
		}
	}
	if _, ok := PrimaryPowerSlot434(3, 0); ok {
		t.Fatal("invented hunter mana mapping")
	}
}
