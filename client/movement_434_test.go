package client

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"testing"
)

// Independent minimal ground fixtures transcribed directly from the three
// TCPP sequences. No production codec or sequence table constructs these bytes.
func groundMovementFixture434(op uint16, g uint64, flags uint32, ts uint32, p Position434) []byte {
	w := &rosterFixture434{}
	switch op {
	case 0x7814:
		w.value(p.Y)
		w.value(p.Z)
		w.value(p.X)
		w.mask(g, 5)
		w.mask(g, 2)
		w.mask(g, 0)
		w.bits(0, 1)
		if flags == 0 {
			w.bits(1, 1)
		} else {
			w.bits(0, 1)
		}
		w.mask(g, 7)
		w.mask(g, 3)
		w.mask(g, 1)
		w.bits(0, 1)
		w.mask(g, 6)
		w.bits(0, 1)
		w.bits(1, 1)
		w.mask(g, 4)
		w.bits(0, 1)
		w.bits(0, 1)
		w.bits(1, 1)
		w.bits(1, 1)
		w.bits(0, 1)
		if flags != 0 {
			w.bits(uint64(flags), 30)
		}
		w.flush()
		for _, i := range []uint{2, 4, 6, 1, 7, 3, 5, 0} {
			w.seq(g, i)
		}
		w.value(p.Orientation)
		w.value(ts)
	case 0x320a:
		w.value(p.X)
		w.value(p.Y)
		w.value(p.Z)
		w.mask(g, 3)
		w.mask(g, 6)
		w.bits(1, 1)
		w.bits(0, 1)
		w.bits(0, 1)
		w.mask(g, 7)
		if flags == 0 {
			w.bits(1, 1)
		} else {
			w.bits(0, 1)
		}
		w.mask(g, 5)
		w.bits(0, 1)
		w.bits(1, 1)
		w.bits(0, 1)
		w.bits(0, 1)
		w.mask(g, 4)
		w.mask(g, 1)
		w.bits(0, 1)
		w.mask(g, 2)
		w.mask(g, 0)
		w.bits(1, 1)
		if flags != 0 {
			w.bits(uint64(flags), 30)
		}
		w.flush()
		for _, i := range []uint{6, 3, 0, 4, 2, 1, 5, 7} {
			w.seq(g, i)
		}
		w.value(ts)
		w.value(p.Orientation)
	case 0x3914:
		w.value(p.Z)
		w.value(p.X)
		w.value(p.Y)
		w.bits(1, 1)
		w.bits(0, 1)
		w.bits(0, 1)
		w.bits(1, 1)
		w.bits(0, 1)
		for _, i := range []uint{7, 1, 0, 4, 2} {
			w.mask(g, i)
		}
		w.bits(0, 1)
		w.mask(g, 5)
		w.mask(g, 3)
		w.bits(1, 1)
		w.bits(0, 1)
		w.bits(0, 1)
		w.mask(g, 6)
		if flags == 0 {
			w.bits(1, 1)
		} else {
			w.bits(0, 1)
		}
		if flags != 0 {
			w.bits(uint64(flags), 30)
		}
		w.flush()
		for _, i := range []uint{3, 6, 1, 7, 2, 5, 0, 4} {
			w.seq(g, i)
		}
		w.value(p.Orientation)
		w.value(ts)
	}
	return w.Bytes()
}
func TestMovement434GroundFixtures(t *testing.T) {
	if cataMoveStartForward != 0x7814 || cataMoveStop != 0x320a || cataMoveHeartbeat != 0x3914 || cataMoveUpdate != 0x79a2 || cataSetActiveMover != 0x3314 {
		t.Fatal("opcode mismatch")
	}
	for _, guid := range []uint64{17, 0x0102030405060708, 0xff00000000000100} {
		for _, op := range []uint16{0x7814, 0x320a, 0x3914} {
			m := Movement434{GUID: guid, Position: Position434{12.25, -3.5, 6.75, 0.5}, Flags: 1, Timestamp: 0x12345678, HasTimestamp: true, HasOrientation: true}
			if op == 0x320a {
				m.Flags = 0
			}
			want := groundMovementFixture434(op, guid, m.Flags, m.Timestamp, m.Position)
			got, err := encodeMovement434(op, m)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("op %x got %x want %x: %v", op, got, want, err)
			}
			decoded, err := decodeMovement434(op, want)
			if err != nil || !reflect.DeepEqual(decoded, m) {
				t.Fatalf("decoded %+v want %+v: %v", decoded, m, err)
			}
			for n := 0; n < len(want); n++ {
				if _, err := decodeMovement434(op, want[:n]); err == nil {
					t.Fatalf("truncated %x/%d", op, n)
				}
			}
			if _, err := decodeMovement434(op, append(want, 0)); err == nil {
				t.Fatal("trailing byte")
			}
		}
	}
	if got := activeMover434(17); !bytes.Equal(got, []byte{0x10, 0x10}) {
		t.Fatalf("active GUID %x", got)
	}
}
func TestMovement434ConditionalFields(t *testing.T) {
	pitch, elevation := float32(0.25), float32(0.5)
	time2, vehicle := uint32(77), uint32(99)
	m := Movement434{GUID: 0x0807060504030201, Position: Position434{1, 2, 3, 0.75}, Flags: 0x02100801, Flags2: 0x10, Timestamp: 42, HasTimestamp: true, HasOrientation: true, Pitch: &pitch, SplineElevation: &elevation, Fall: &Fall434{Time: 12, VerticalSpeed: 3, Direction: &FallDirection434{4, 0, 1}}, Transport: &Transport434{GUID: 0x1122334455667788, Position: Position434{2, 3, 4, 1}, Seat: -1, Time: 88, Time2: &time2, VehicleID: &vehicle}}
	for _, op := range []uint16{0x7814, 0x320a, 0x3914, 0x79a2} {
		b, err := encodeMovement434(op, m)
		if err != nil {
			t.Fatal(err)
		}
		got, err := decodeMovement434(op, b)
		if err != nil || !reflect.DeepEqual(got, m) {
			t.Fatalf("op %x %+v %v", op, got, err)
		}
		for i := 0; i < len(b); i++ {
			if _, err := decodeMovement434(op, b[:i]); err == nil {
				t.Fatalf("rich truncated %x/%d", op, i)
			}
		}
	}
	for _, bad := range []Movement434{{GUID: 1, Flags: 1 << 30}, {GUID: 1, Flags2: 1 << 12}, {GUID: 1, Position: Position434{X: float32(math.NaN())}}, {GUID: 0}} {
		if _, err := encodeMovement434(0x7814, bad); err == nil {
			t.Fatal("invalid movement accepted")
		}
	}
}
func TestMovement434StoreAndProof(t *testing.T) {
	s := WorldState434Result{Store: ObjectStore434{PlayerGUID: 17}, Opcodes: map[uint16]int{}, WorldVariables: map[uint32]int32{}}
	if err := s.Store.ApplyUpdate(updateFixture434(livingFixture434(17, 4, false))); err != nil {
		t.Fatal(err)
	}
	p := s.Store.objects[17]
	initial := p.Movement.clone()
	if initial.RunSpeed != 7 || initial.Timestamp != 12345 {
		t.Fatal(initial)
	}
	target, err := tinyDestination434(initial)
	if err != nil {
		t.Fatal(err)
	}
	target.HasTimestamp = true
	target.Timestamp = 222
	target.RunSpeed = 0
	b, err := encodeMovement434(0x79a2, target)
	if err != nil {
		t.Fatal(err)
	}
	before := s.Store.Objects()
	if s.Store.ApplyMovement(b[:len(b)-1]) == nil || !reflect.DeepEqual(before, s.Store.Objects()) {
		t.Fatal("malformed update mutated store")
	}
	if err := s.Store.ApplyMovement(b); err != nil {
		t.Fatal(err)
	}
	if p.Position.X != target.Position.X || p.Movement.Moving() || p.Fields[0x30] != 7 {
		t.Fatal(p)
	}
	s.Login = CharacterLogin434Result{Character: Character434{Name: "Synthetic", GUID: 17}, Map: 1, X: target.Position.X, Y: target.Position.Y, Z: target.Position.Z, Orientation: target.Position.Orientation}
	a := MovementAttempt434{Initial: initial, Stop: target, World: s}
	if _, err := ValidateMovementProof434(a, s); err != nil {
		t.Fatal(err)
	}
	a.Stop.Position.X += 0.5
	if _, err := ValidateMovementProof434(a, s); err == nil {
		t.Fatal("heartbeat mistaken for stop proof")
	}
	// Removal prevents late movement packets from recreating an old lifetime.
	if err := s.Store.ApplyDestroy(append(binary.LittleEndian.AppendUint64(nil, 17), 0)); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.ApplyMovement(b); err != nil || len(s.Store.Objects()) != 0 {
		t.Fatal("movement resurrected object")
	}
}
func TestMovement434TinyController(t *testing.T) {
	s := WorldState434Result{Store: ObjectStore434{PlayerGUID: 17}, Opcodes: map[uint16]int{}, WorldVariables: map[uint32]int32{}}
	s.Store.ApplyUpdate(updateFixture434(livingFixture434(17, 4, false)))
	s.Login = CharacterLogin434Result{Map: 1, X: 1.25, Y: -2.5, Z: 3.75, Orientation: 0.5}
	result := MovementAttempt434{}
	controller := tinyMovement434{world: &s, result: &result}
	var ops []uint16
	send := func(op uint16, b []byte) error {
		ops = append(ops, op)
		if op != 0x3314 {
			_, err := decodeMovement434(op, b)
			return err
		}
		return nil
	}
	for _, now := range []uint32{100, 1100, 1300, 1500, 2500} {
		if err := controller.tick(now, send); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(ops, []uint16{0x3314, 0x7814, 0x3914, 0x320a}) || controller.stage != 4 {
		t.Fatal(ops, controller.stage)
	}
	if s.Store.objects[17].Position.X != 1.25 {
		t.Fatal("outbound proposal changed authoritative store")
	}
	bad := result.Initial
	bad.Flags = 0x100000
	if _, err := tinyDestination434(bad); err == nil {
		t.Fatal("swimming accepted")
	}
}
