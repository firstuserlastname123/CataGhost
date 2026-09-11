package client

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const (
	cataMoveStartForward uint16 = 0x7814
	cataMoveStop         uint16 = 0x320a
	cataMoveHeartbeat    uint16 = 0x3914
	cataMoveUpdate       uint16 = 0x79a2
	cataSetActiveMover   uint16 = 0x3314
	moveForward434       uint32 = 1
)

// Movement434 holds server-reported movement or a separately identified outbound
// proposal. Sending a proposal never updates the authoritative object store.
type Movement434 struct {
	GUID                                                     uint64
	Position                                                 Position434
	Flags                                                    uint32
	Flags2                                                   uint16
	Timestamp                                                uint32
	HasTimestamp, HasOrientation, Spline, HeightChangeFailed bool
	Transport                                                *Transport434
	Pitch, SplineElevation                                   *float32
	Fall                                                     *Fall434
	RunSpeed                                                 float32 // available in a create-object snapshot, not MSG_MOVE_*
}
type Transport434 struct {
	GUID             uint64
	Position         Position434
	Seat             int8
	Time             uint32
	Time2, VehicleID *uint32
}
type Fall434 struct {
	Time          uint32
	VerticalSpeed float32
	Direction     *FallDirection434
}
type FallDirection434 struct{ HorizontalSpeed, SinAngle, CosAngle float32 }

func (m Movement434) Moving() bool { return m.Flags&0x026018ff != 0 }

// Literal order from TCPP MovementStructures.cpp, build 15595. Each opcode has
// its own layout. Transport/fall tokens are conditional; they are not padding.
var movementSequences434 = map[uint16]string{
	0x7814: `PositionY PositionZ PositionX HasGuidByte5 HasGuidByte2 HasGuidByte0 ZeroBit HasMovementFlags HasGuidByte7 HasGuidByte3 HasGuidByte1 HasOrientation HasGuidByte6 HasSpline HasSplineElevation HasGuidByte4 HasTransportData HasTimestamp HasPitch HasMovementFlags2 HasFallData MovementFlags HasTransportGuidByte3 HasTransportGuidByte4 HasTransportGuidByte6 HasTransportGuidByte2 HasTransportGuidByte5 HasTransportGuidByte0 HasTransportGuidByte7 HasTransportGuidByte1 HasVehicleId HasTransportTime2 HasFallDirection MovementFlags2 GuidByte2 GuidByte4 GuidByte6 GuidByte1 GuidByte7 GuidByte3 GuidByte5 GuidByte0 FallVerticalSpeed FallHorizontalSpeed FallSinAngle FallCosAngle FallTime TransportGuidByte3 TransportPositionY TransportPositionZ TransportGuidByte1 TransportGuidByte4 TransportGuidByte7 TransportOrientation TransportGuidByte2 TransportPositionX TransportGuidByte5 TransportVehicleId TransportTime TransportGuidByte6 TransportGuidByte0 TransportSeat TransportTime2 SplineElevation Pitch Orientation Timestamp`,
	0x320a: `PositionX PositionY PositionZ HasGuidByte3 HasGuidByte6 HasSplineElevation HasSpline HasOrientation HasGuidByte7 HasMovementFlags HasGuidByte5 HasFallData HasMovementFlags2 HasTransportData HasTimestamp HasGuidByte4 HasGuidByte1 ZeroBit HasGuidByte2 HasGuidByte0 HasPitch HasTransportGuidByte7 HasTransportGuidByte4 HasTransportGuidByte1 HasTransportGuidByte5 HasTransportTime2 HasVehicleId HasTransportGuidByte3 HasTransportGuidByte6 HasTransportGuidByte0 HasTransportGuidByte2 MovementFlags MovementFlags2 HasFallDirection GuidByte6 GuidByte3 GuidByte0 GuidByte4 GuidByte2 GuidByte1 GuidByte5 GuidByte7 TransportGuidByte4 TransportGuidByte7 TransportTime TransportSeat TransportPositionZ TransportVehicleId TransportGuidByte2 TransportGuidByte0 TransportPositionY TransportGuidByte1 TransportGuidByte3 TransportTime2 TransportPositionX TransportOrientation TransportGuidByte5 TransportGuidByte6 Timestamp Orientation Pitch SplineElevation FallSinAngle FallCosAngle FallHorizontalSpeed FallVerticalSpeed FallTime`,
	0x3914: `PositionZ PositionX PositionY HasPitch HasTimestamp HasFallData HasMovementFlags2 HasTransportData HasGuidByte7 HasGuidByte1 HasGuidByte0 HasGuidByte4 HasGuidByte2 HasOrientation HasGuidByte5 HasGuidByte3 HasSplineElevation HasSpline ZeroBit HasGuidByte6 HasMovementFlags HasVehicleId HasTransportGuidByte4 HasTransportGuidByte2 HasTransportTime2 HasTransportGuidByte5 HasTransportGuidByte7 HasTransportGuidByte6 HasTransportGuidByte0 HasTransportGuidByte3 HasTransportGuidByte1 HasFallDirection MovementFlags MovementFlags2 GuidByte3 GuidByte6 GuidByte1 GuidByte7 GuidByte2 GuidByte5 GuidByte0 GuidByte4 TransportPositionZ TransportSeat TransportOrientation TransportGuidByte4 TransportPositionY TransportTime TransportPositionX TransportGuidByte5 TransportGuidByte1 TransportGuidByte3 TransportGuidByte7 TransportVehicleId TransportTime2 TransportGuidByte2 TransportGuidByte0 TransportGuidByte6 Orientation FallVerticalSpeed FallTime FallHorizontalSpeed FallCosAngle FallSinAngle Pitch SplineElevation Timestamp`,
	0x79a2: `HasFallData HasGuidByte3 HasGuidByte6 HasMovementFlags2 HasSpline HasTimestamp HasGuidByte0 HasGuidByte1 MovementFlags2 HasGuidByte7 HasMovementFlags HasOrientation HasGuidByte2 HasSplineElevation HasHeightChangeFailed HasGuidByte4 HasFallDirection HasGuidByte5 HasTransportData MovementFlags HasTransportGuidByte3 HasVehicleId HasTransportGuidByte6 HasTransportGuidByte1 HasTransportGuidByte7 HasTransportGuidByte0 HasTransportGuidByte4 HasTransportTime2 HasTransportGuidByte5 HasTransportGuidByte2 HasPitch FlushBits GuidByte5 FallHorizontalSpeed FallSinAngle FallCosAngle FallVerticalSpeed FallTime SplineElevation GuidByte7 PositionY GuidByte3 TransportVehicleId TransportGuidByte6 TransportSeat TransportGuidByte5 TransportPositionX TransportGuidByte1 TransportOrientation TransportGuidByte2 TransportTime2 TransportGuidByte0 TransportPositionZ TransportGuidByte7 TransportGuidByte4 TransportGuidByte3 TransportPositionY TransportTime GuidByte4 PositionX GuidByte6 PositionZ Timestamp GuidByte2 Pitch GuidByte0 Orientation GuidByte1`,
}

type movementIO434 struct {
	r     *objectReader434
	b     []byte
	bit   int
	write bool
}

func (c *movementIO434) bits(v uint32, n int) uint32 {
	if !c.write {
		return c.r.bits(n)
	}
	for i := n - 1; i >= 0; i-- {
		if c.bit%8 == 0 {
			c.b = append(c.b, 0)
		}
		c.b[c.bit/8] |= byte(v>>uint(i)&1) << uint(7-c.bit%8)
		c.bit++
	}
	return v
}
func (c *movementIO434) align() {
	if c.write {
		c.bit = (c.bit + 7) / 8 * 8
	} else {
		c.r.align()
	}
}
func (c *movementIO434) value(v uint32, n int) uint32 {
	c.align()
	if !c.write {
		switch n {
		case 1:
			return uint32(c.r.u8())
		case 4:
			return c.r.u32()
		}
	}
	for i := 0; i < n; i++ {
		c.b = append(c.b, byte(v>>uint(i*8)))
	}
	c.bit += 8 * n
	return v
}
func (c *movementIO434) f(v *float32) { *v = math.Float32frombits(c.value(math.Float32bits(*v), 4)) }
func (c *movementIO434) presence(v bool, invert bool) bool {
	var b uint32
	if v != invert {
		b = 1
	}
	return (c.bits(b, 1) != 0) != invert
}

func movementCodec434(op uint16, m Movement434, c *movementIO434) (Movement434, error) {
	sequence, ok := movementSequences434[op]
	if !ok {
		return m, fmt.Errorf("unsupported movement opcode 0x%04X", op)
	}
	var g, t [8]byte
	binary.LittleEndian.PutUint64(g[:], m.GUID)
	if m.Transport != nil {
		binary.LittleEndian.PutUint64(t[:], m.Transport.GUID)
	}
	hasFlags, hasFlags2 := m.Flags != 0, m.Flags2 != 0
	for _, token := range strings.Fields(sequence) {
		// The four GUID token families use the same XOR-1 primitive but distinct orders.
		handled := false
		for _, family := range []struct {
			prefix          string
			mask, transport bool
		}{{"HasGuidByte", true, false}, {"HasTransportGuidByte", true, true}, {"GuidByte", false, false}, {"TransportGuidByte", false, true}} {
			if !strings.HasPrefix(token, family.prefix) {
				continue
			}
			handled = true
			if family.transport && m.Transport == nil {
				break
			}
			i := int(token[len(token)-1] - '0')
			guid := &g
			if family.transport {
				guid = &t
			}
			if family.mask {
				v := c.presence(guid[i] != 0, false)
				if !c.write {
					if v {
						guid[i] = 1
					} else {
						guid[i] = 0
					}
				}
			} else if guid[i] != 0 {
				guid[i] = byte(c.value(uint32(guid[i]^1), 1)) ^ 1
			}
			break
		}
		if handled {
			continue
		}
		switch token {
		case "ZeroBit":
			c.bits(0, 1)
		case "FlushBits":
			c.align()
		case "HasMovementFlags":
			hasFlags = c.presence(hasFlags, true)
		case "HasMovementFlags2":
			hasFlags2 = c.presence(hasFlags2, true)
		case "HasTimestamp":
			m.HasTimestamp = c.presence(m.HasTimestamp, true)
		case "HasOrientation":
			m.HasOrientation = c.presence(m.HasOrientation, true)
		case "HasSpline":
			m.Spline = c.presence(m.Spline, false)
		case "HasHeightChangeFailed":
			m.HeightChangeFailed = c.presence(m.HeightChangeFailed, false)
		case "HasTransportData":
			if c.presence(m.Transport != nil, false) {
				if m.Transport == nil {
					m.Transport = &Transport434{}
				}
			} else {
				m.Transport = nil
			}
		case "HasVehicleId":
			if m.Transport != nil {
				if c.presence(m.Transport.VehicleID != nil, false) {
					if m.Transport.VehicleID == nil {
						m.Transport.VehicleID = new(uint32)
					}
				} else {
					m.Transport.VehicleID = nil
				}
			}
		case "HasTransportTime2":
			if m.Transport != nil {
				if c.presence(m.Transport.Time2 != nil, false) {
					if m.Transport.Time2 == nil {
						m.Transport.Time2 = new(uint32)
					}
				} else {
					m.Transport.Time2 = nil
				}
			}
		case "HasPitch":
			if c.presence(m.Pitch != nil, true) {
				if m.Pitch == nil {
					m.Pitch = new(float32)
				}
			} else {
				m.Pitch = nil
			}
		case "HasSplineElevation":
			if c.presence(m.SplineElevation != nil, true) {
				if m.SplineElevation == nil {
					m.SplineElevation = new(float32)
				}
			} else {
				m.SplineElevation = nil
			}
		case "HasFallData":
			if c.presence(m.Fall != nil, false) {
				if m.Fall == nil {
					m.Fall = &Fall434{}
				}
			} else {
				m.Fall = nil
			}
		case "HasFallDirection":
			if m.Fall != nil {
				if c.presence(m.Fall.Direction != nil, false) {
					if m.Fall.Direction == nil {
						m.Fall.Direction = &FallDirection434{}
					}
				} else {
					m.Fall.Direction = nil
				}
			}
		case "MovementFlags":
			if hasFlags {
				m.Flags = c.bits(m.Flags, 30)
			}
		case "MovementFlags2":
			if hasFlags2 {
				m.Flags2 = uint16(c.bits(uint32(m.Flags2), 12))
			}
		case "Timestamp":
			if m.HasTimestamp {
				m.Timestamp = c.value(m.Timestamp, 4)
			}
		case "PositionX":
			c.f(&m.Position.X)
		case "PositionY":
			c.f(&m.Position.Y)
		case "PositionZ":
			c.f(&m.Position.Z)
		case "Orientation":
			if m.HasOrientation {
				c.f(&m.Position.Orientation)
			}
		case "Pitch":
			if m.Pitch != nil {
				c.f(m.Pitch)
			}
		case "SplineElevation":
			if m.SplineElevation != nil {
				c.f(m.SplineElevation)
			}
		case "TransportPositionX":
			if m.Transport != nil {
				c.f(&m.Transport.Position.X)
			}
		case "TransportPositionY":
			if m.Transport != nil {
				c.f(&m.Transport.Position.Y)
			}
		case "TransportPositionZ":
			if m.Transport != nil {
				c.f(&m.Transport.Position.Z)
			}
		case "TransportOrientation":
			if m.Transport != nil {
				c.f(&m.Transport.Position.Orientation)
			}
		case "TransportSeat":
			if m.Transport != nil {
				m.Transport.Seat = int8(c.value(uint32(uint8(m.Transport.Seat)), 1))
			}
		case "TransportTime":
			if m.Transport != nil {
				m.Transport.Time = c.value(m.Transport.Time, 4)
			}
		case "TransportTime2":
			if m.Transport != nil && m.Transport.Time2 != nil {
				*m.Transport.Time2 = c.value(*m.Transport.Time2, 4)
			}
		case "TransportVehicleId":
			if m.Transport != nil && m.Transport.VehicleID != nil {
				*m.Transport.VehicleID = c.value(*m.Transport.VehicleID, 4)
			}
		case "FallTime":
			if m.Fall != nil {
				m.Fall.Time = c.value(m.Fall.Time, 4)
			}
		case "FallVerticalSpeed":
			if m.Fall != nil {
				c.f(&m.Fall.VerticalSpeed)
			}
		case "FallHorizontalSpeed":
			if m.Fall != nil && m.Fall.Direction != nil {
				c.f(&m.Fall.Direction.HorizontalSpeed)
			}
		case "FallSinAngle":
			if m.Fall != nil && m.Fall.Direction != nil {
				c.f(&m.Fall.Direction.SinAngle)
			}
		case "FallCosAngle":
			if m.Fall != nil && m.Fall.Direction != nil {
				c.f(&m.Fall.Direction.CosAngle)
			}
		default:
			return m, fmt.Errorf("unrecognized movement element %s", token)
		}
	}
	m.GUID = binary.LittleEndian.Uint64(g[:])
	if m.Transport != nil {
		m.Transport.GUID = binary.LittleEndian.Uint64(t[:])
	}
	if !c.write {
		if err := c.r.done(); err != nil {
			return m, err
		}
	}
	if m.GUID == 0 || !finitePosition434(&m.Position) || m.Flags >= 1<<30 || m.Flags2 >= 1<<12 {
		return m, fmt.Errorf("invalid movement identity/position/flags")
	}
	if m.Transport != nil && !finitePosition434(&m.Transport.Position) {
		return m, fmt.Errorf("invalid transport position")
	}
	for _, v := range []*float32{m.Pitch, m.SplineElevation} {
		if v != nil && (math.IsNaN(float64(*v)) || math.IsInf(float64(*v), 0)) {
			return m, fmt.Errorf("invalid optional movement float")
		}
	}
	if m.Fall != nil {
		floats := []float32{m.Fall.VerticalSpeed}
		if m.Fall.Direction != nil {
			d := m.Fall.Direction
			floats = append(floats, d.HorizontalSpeed, d.SinAngle, d.CosAngle)
		}
		for _, v := range floats {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return m, fmt.Errorf("invalid fall float")
			}
		}
	}
	return m, nil
}
func encodeMovement434(op uint16, m Movement434) ([]byte, error) {
	c := &movementIO434{write: true}
	_, err := movementCodec434(op, m.clone(), c)
	return c.b, err
}
func decodeMovement434(op uint16, b []byte) (Movement434, error) {
	return movementCodec434(op, Movement434{}, &movementIO434{r: newObjectReader434(b)})
}
func (m Movement434) clone() Movement434 {
	if m.Transport != nil {
		v := *m.Transport
		m.Transport = &v
		if v.Time2 != nil {
			x := *v.Time2
			m.Transport.Time2 = &x
		}
		if v.VehicleID != nil {
			x := *v.VehicleID
			m.Transport.VehicleID = &x
		}
	}
	if m.Pitch != nil {
		x := *m.Pitch
		m.Pitch = &x
	}
	if m.SplineElevation != nil {
		x := *m.SplineElevation
		m.SplineElevation = &x
	}
	if m.Fall != nil {
		x := *m.Fall
		m.Fall = &x
		if x.Direction != nil {
			v := *x.Direction
			m.Fall.Direction = &v
		}
	}
	return m
}
func activeMover434(guid uint64) []byte {
	c := &movementIO434{write: true}
	var g [8]byte
	binary.LittleEndian.PutUint64(g[:], guid)
	for _, i := range []int{7, 2, 1, 0, 4, 5, 6, 3} {
		c.presence(g[i] != 0, false)
	}
	for _, i := range []int{3, 2, 4, 0, 5, 1, 6, 7} {
		if g[i] != 0 {
			c.value(uint32(g[i]^1), 1)
		}
	}
	return c.b
}
