package client

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestWorldState434IdentityAndWorldVariables(t *testing.T) {
	s := WorldState434Result{Store: ObjectStore434{PlayerGUID: 17}, Opcodes: make(map[uint16]int), WorldVariables: make(map[uint32]int32)}
	s.Login = CharacterLogin434Result{Map: 1, X: 1.25, Y: -2.5, Z: 3.75, Orientation: 0.5}
	if ready, err := s.playerReady(); ready || err != nil {
		t.Fatal("empty store ready")
	}
	if err := s.observe(loginPacket434{op: 0x4715, body: updateFixture434(livingFixture434(17, 4, false))}); err != nil {
		t.Fatal(err)
	}
	if ready, err := s.playerReady(); !ready || err != nil {
		t.Fatal(ready, err)
	}
	s.Login.X++
	if _, err := s.playerReady(); err == nil {
		t.Fatal("position mismatch accepted")
	}
	s.Login.X--
	w := &rosterFixture434{}
	w.value(uint32(1))
	w.value(uint32(10))
	w.value(uint32(11))
	w.value(uint16(1))
	w.value(uint32(77))
	w.value(int32(-4))
	if err := s.observe(loginPacket434{op: 0x4c15, body: w.Bytes()}); err != nil {
		t.Fatal(err)
	}
	if s.WorldVariables[77] != -4 || s.Area != 10 || len(s.Store.Objects()) != 1 {
		t.Fatal("world initialization lost objects")
	}
	b := binary.LittleEndian.AppendUint32(nil, 77)
	b = binary.LittleEndian.AppendUint32(b, 5)
	b = append(b, 0)
	if err := s.observe(loginPacket434{op: 0x4816, body: b}); err != nil || s.WorldVariables[77] != 5 {
		t.Fatal(err)
	}
	for _, op := range []uint16{0x4c15, 0x4816, 0x4724, 0x79b1} {
		if err := s.observe(loginPacket434{op: op, body: []byte{1}}); err == nil {
			t.Fatalf("truncated %x", op)
		}
	}
}

func TestWorldState434CancelTimeout(t *testing.T) {
	for _, immediate := range []bool{false, true} {
		left, right := net.Pipe()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		if immediate {
			cancel()
		}
		var login CharacterLogin434Result
		err := awaitObservedLogin434(ctx, &worldWire434{conn: left}, "SYNTHETIC", "localhost:1", &login, openInstance434, func(loginPacket434) error { return nil }, func() (bool, error) { return false, nil })
		cancel()
		if err == nil {
			t.Fatal("cancellation/timeout succeeded")
		}
		var b [1]byte
		right.SetReadDeadline(time.Now().Add(time.Second))
		n, _ := right.Read(b[:])
		right.Close()
		if n != 0 {
			t.Fatal("unexpected outbound packet")
		}
	}
}

func TestObject434NonlivingTypes(t *testing.T) {
	for typ := uint8(0); typ <= 8; typ++ {
		if typ == 3 || typ == 4 {
			continue
		}
		w := &rosterFixture434{}
		w.value(uint8(1))
		w.Write(packedFixture434(uint64(typ) + 100))
		w.value(typ)
		// rotation + animation kit + pause times + stationary + area data + time
		w.bits(0x30, 8)
		w.bits(2, 24)
		w.bits(0x0d, 6)
		w.bits(0, 3)
		w.flush()
		w.value([2]uint32{1, 2})
		w.value(uint64(42))
		w.Write(make([]byte, 65))
		w.value([4]float32{0.5, 1.25, -2.5, 3.75})
		w.value([3]uint16{1, 2, 3})
		w.value(uint32(123))
		w.Write(fieldsFixture434(map[uint16]uint32{5: 999}))
		s := ObjectStore434{}
		if err := s.ApplyUpdate(updateFixture434(w.Bytes())); err != nil {
			t.Fatalf("type %d: %v", typ, err)
		}
		if o := s.Objects()[0]; o.Type != typ || o.Position.X != 1.25 || o.Fields[5] != 999 {
			t.Fatal(o)
		}
	}
}
