package client

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	cataUpdateObject     = 0x4715
	cataDestroyObject    = 0x4724
	cataInitWorldStates  = 0x4c15
	cataUpdateWorldState = 0x4816
	cataNewWorld         = 0x79b1
)

// Position434 is a server-reported snapshot, not a movement instruction.
type Position434 struct{ X, Y, Z, Orientation float32 }

type objectDelta434 struct {
	kind     uint8
	guid     uint64
	typeID   uint8
	fields   map[uint16]uint32
	position *Position434
	movement *Movement434
	self     bool
	removed  []uint64
}

type objectReader434 struct{ rosterReader434 }

func newObjectReader434(b []byte) *objectReader434 { return &objectReader434{rosterReader434{b: b}} }
func (r *objectReader434) flag() bool              { return r.bits(1) != 0 }
func (r *objectReader434) align()                  { r.bit = (r.bit + 7) / 8 * 8 }
func (r *objectReader434) u16() uint16 {
	b := r.take(2)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint16(b)
}
func (r *objectReader434) u64() uint64 {
	b := r.take(8)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint64(b)
}
func (r *objectReader434) mask(g *[8]byte, order ...int) {
	for _, i := range order {
		g[i] = byte(r.bits(1))
	}
}
func (r *objectReader434) seq(g *[8]byte, order ...int) {
	for _, i := range order {
		r.guid(g, i)
	}
}
func (r *objectReader434) packed() uint64 {
	m := r.u8()
	var g [8]byte
	for i := range g {
		if m&(1<<uint(i)) != 0 {
			g[i] = r.u8()
		}
	}
	return binary.LittleEndian.Uint64(g[:])
}
func (r *objectReader434) done() error {
	if r.err != nil {
		return r.err
	}
	if r.bit != len(r.b)*8 {
		return fmt.Errorf("unexpected trailing bytes")
	}
	return nil
}
func (r *objectReader434) bounded(n, size int) bool {
	if n < 0 || size <= 0 || n > (len(r.b)-r.bit/8)/size {
		r.err = fmt.Errorf("count exceeds remaining packet")
		return false
	}
	return r.err == nil
}
func finitePosition434(p *Position434) bool {
	for _, v := range []float32{p.X, p.Y, p.Z, p.Orientation} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	return true
}

// parseObjectUpdate434 is the inverse of TCPP UpdateData::BuildPacket and
// Object::Build{Create,Values}UpdateBlockForPlayer (build 15595).
// Parsing finishes before any store mutation, including out-of-range removals.
func parseObjectUpdate434(b []byte) (uint16, []objectDelta434, error) {
	r := newObjectReader434(b)
	mapID := r.u16()
	count := int(r.u32())
	if !r.bounded(count, 2) {
		return 0, nil, fmt.Errorf("object update: %w", r.err)
	}
	var deltas []objectDelta434
	for i := 0; i < count; i++ {
		d := objectDelta434{kind: r.u8()}
		switch d.kind {
		case 3:
			n := int(r.u32())
			if !r.bounded(n, 1) {
				return 0, nil, r.err
			}
			for j := 0; j < n; j++ {
				d.removed = append(d.removed, r.packed())
			}
		case 0, 1, 2:
			d.guid = r.packed()
			if d.guid == 0 {
				return 0, nil, fmt.Errorf("object block %d has empty GUID", i)
			}
			if d.kind != 0 {
				d.typeID = r.u8()
				if d.typeID > 8 {
					return 0, nil, fmt.Errorf("unknown object type %d", d.typeID)
				}
				d.position, d.self, d.movement = r.movement(d.guid)
			}
			n := int(r.u8())
			if !r.bounded(n, 4) {
				return 0, nil, r.err
			}
			mask := make([]uint32, n)
			for j := range mask {
				mask[j] = r.u32()
			}
			d.fields = make(map[uint16]uint32)
			for j, m := range mask {
				for bit := 0; bit < 32; bit++ {
					if m&(1<<uint(bit)) != 0 {
						d.fields[uint16(j*32+bit)] = r.u32()
					}
				}
			}
		default:
			return 0, nil, fmt.Errorf("unknown update type %d at block %d", d.kind, i)
		}
		if r.err != nil {
			return 0, nil, fmt.Errorf("object block %d: %w", i, r.err)
		}
		deltas = append(deltas, d)
	}
	if err := r.done(); err != nil {
		return 0, nil, fmt.Errorf("object update: %w", err)
	}
	return mapID, deltas, nil
}

type spline434 struct {
	active, effect, vertical bool
	nodes                    int
	facing                   uint32
	target                   [8]byte
}

func (r *objectReader434) splineBits() spline434 {
	s := spline434{active: r.flag()}
	if !s.active {
		return s
	}
	r.bits(2)
	s.effect = r.flag()
	s.nodes = int(r.bits(22))
	s.facing = r.bits(2)
	if s.facing == 2 {
		r.mask(&s.target, 4, 3, 7, 2, 6, 1, 0, 5)
	}
	s.vertical = r.flag()
	r.bits(25)
	return s
}
func (r *objectReader434) splineData(s spline434) {
	if s.active {
		if s.vertical {
			r.f32()
		}
		r.u32()
		if s.facing == 0 {
			r.f32()
		} else if s.facing == 2 {
			r.seq(&s.target, 5, 3, 7, 1, 6, 4, 2, 0)
		}
		if !r.bounded(s.nodes, 12) {
			return
		}
		r.take(s.nodes * 12) // nodes Z,X,Y
		if s.facing == 1 {
			r.take(12)
		} // facing spot X,Z,Y
		r.f32()
		r.u32()
		if s.effect {
			r.u32()
		}
		r.f32()
	}
	r.take(16) // destination Z,X,Y and spline ID, even for finalized splines
}

// movement consumes the full build-15595 create movement block. Optional
// transport/spline/animation data is decoded for framing but not simulated.
func (r *objectReader434) movement(outerGUID uint64) (*Position434, bool, *Movement434) {
	var state *Movement434
	r.flag()
	r.flag()
	rotation := r.flag()
	anim := r.flag()
	victim := r.flag()
	self := r.flag()
	vehicle := r.flag()
	living := r.flag()
	pauses := int(r.bits(24))
	r.flag()
	goTransport := r.flag()
	stationary := r.flag()
	area := r.flag()
	r.flag()
	serverTime := r.flag()
	var g, t, gt, v [8]byte
	var orientation, pitch, spline, fall, elevation, transport, hasTime, time2, vehicleID, direction bool
	var goTime2, goVehicleID bool
	var s spline434
	if living {
		state = &Movement434{GUID: outerGUID, HasOrientation: true}
		flags := !r.flag()
		orientation = !r.flag()
		r.mask(&g, 7, 3, 2)
		if flags {
			state.Flags = r.bits(30)
		}
		r.flag()
		pitch = !r.flag()
		spline = r.flag()
		state.Spline = spline
		fall = r.flag()
		elevation = !r.flag()
		r.mask(&g, 5)
		transport = r.flag()
		hasTime = !r.flag()
		if transport {
			r.mask(&t, 1)
			time2 = r.flag()
			r.mask(&t, 4, 0, 6)
			vehicleID = r.flag()
			r.mask(&t, 7, 5, 3, 2)
		}
		r.mask(&g, 4)
		if spline {
			s = r.splineBits()
		}
		r.mask(&g, 6)
		if fall {
			direction = r.flag()
		}
		r.mask(&g, 0, 1)
		r.flag()
		if !r.flag() {
			state.Flags2 = uint16(r.bits(12))
		}
	}
	if goTransport {
		r.mask(&gt, 5)
		goVehicleID = r.flag()
		r.mask(&gt, 0, 3, 6, 1, 4, 2)
		goTime2 = r.flag()
		r.mask(&gt, 7)
	}
	if victim {
		r.mask(&v, 2, 7, 0, 4, 5, 6, 1, 3)
	}
	var kits [3]bool
	if anim {
		for i := range kits {
			kits[i] = !r.flag()
		}
	}
	r.align()
	if !r.bounded(pauses, 4) {
		return nil, self, state
	}
	r.take(pauses * 4)
	var p *Position434
	if living {
		p = &Position434{}
		r.seq(&g, 4)
		r.f32()
		if fall {
			state.Fall = &Fall434{}
			if direction {
				state.Fall.Direction = &FallDirection434{HorizontalSpeed: r.f32(), SinAngle: r.f32(), CosAngle: r.f32()}
			}
			state.Fall.Time = r.u32()
			state.Fall.VerticalSpeed = r.f32()
		}
		r.f32()
		if elevation {
			v := r.f32()
			state.SplineElevation = &v
		}
		if spline {
			r.splineData(s)
		}
		p.Z = r.f32()
		r.seq(&g, 5)
		if transport {
			state.Transport = &Transport434{}
			r.seq(&t, 5, 7)
			state.Transport.Time = r.u32()
			state.Transport.Position.Orientation = r.f32()
			if time2 {
				v := r.u32()
				state.Transport.Time2 = &v
			}
			state.Transport.Position.Y = r.f32()
			state.Transport.Position.X = r.f32()
			r.seq(&t, 3)
			state.Transport.Position.Z = r.f32()
			r.seq(&t, 0)
			if vehicleID {
				v := r.u32()
				state.Transport.VehicleID = &v
			}
			state.Transport.Seat = int8(r.u8())
			r.seq(&t, 1, 6, 2, 4)
			state.Transport.GUID = binary.LittleEndian.Uint64(t[:])
		}
		p.X = r.f32()
		r.f32()
		r.seq(&g, 3, 0)
		r.f32()
		p.Y = r.f32()
		r.seq(&g, 7, 1, 2)
		r.f32()
		if hasTime {
			state.Timestamp = r.u32()
			state.HasTimestamp = true
		}
		r.f32()
		r.seq(&g, 6)
		r.f32()
		if orientation {
			p.Orientation = r.f32()
		}
		state.RunSpeed = r.f32()
		if pitch {
			v := r.f32()
			state.Pitch = &v
		}
		r.f32()
		if r.err == nil && binary.LittleEndian.Uint64(g[:]) != outerGUID {
			r.err = fmt.Errorf("movement GUID differs from object GUID")
		}
	}
	if vehicle {
		r.f32()
		r.u32()
	}
	if goTransport {
		r.seq(&gt, 0, 5)
		if goVehicleID {
			r.u32()
		}
		r.seq(&gt, 3)
		r.f32()
		r.seq(&gt, 4, 6, 1)
		r.u32()
		r.f32()
		r.seq(&gt, 2, 7)
		r.f32()
		r.u8()
		r.f32()
		if goTime2 {
			r.u32()
		}
	}
	if rotation {
		r.u64()
	}
	if area {
		r.take(65)
	} // 16 floats and one byte
	if stationary {
		p = &Position434{Orientation: r.f32(), X: r.f32(), Y: r.f32(), Z: r.f32()}
	}
	if victim {
		r.seq(&v, 4, 0, 3, 5, 7, 6, 2, 1)
	}
	for _, present := range kits {
		if present {
			r.u16()
		}
	}
	if serverTime {
		r.u32()
	}
	if p != nil && !finitePosition434(p) {
		r.err = fmt.Errorf("non-finite object position")
	}
	if state != nil && p != nil {
		state.Position = *p
	}
	return p, self, state
}
