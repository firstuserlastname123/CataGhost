package client

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
)

func petFixture434(t *testing.T) (*petSafetyController434, *Object434, *Object434) {
	c, p, _ := combatFixture434(t)
	guid := uint64(0xf140000400000004)
	p.Fields[0xa] = uint32(guid)
	p.Fields[0xb] = uint32(guid >> 32)
	pet := &Object434{GUID: guid, Type: 3, Created: true, Map: 1, Position: &Position434{X: 2, Y: 3, Z: 4}, Fields: map[uint16]uint32{FieldEntry434: 42721, 0x10: uint32(p.GUID), 0x11: uint32(p.GUID >> 32), FieldHealth434: 50, FieldMaxHealth434: 50, combatFlags434: 8}}
	c.result.Observation.World.Store.objects[guid] = pet
	a := &PetSafetyObservation434{Observation: c.result.Observation}
	pc := &petSafetyController434{result: a}
	pc.query = questProgressController434{result: &a.Observation, done: true}
	return pc, p, pet
}

func petPacketFixture434(guid uint64, reaction, command byte) []byte {
	b := binary.LittleEndian.AppendUint64(nil, guid)
	b = binary.LittleEndian.AppendUint16(b, 2)
	b = binary.LittleEndian.AppendUint32(b, 0)
	b = append(b, reaction, command, 0, 0)
	for _, word := range []uint32{0x07000002, 0x07000001, 0x07000004, 0, 0, 0, 0, 0x06000003, 0x06000001, 0x06000000} {
		b = binary.LittleEndian.AppendUint32(b, word)
	}
	b = append(b, 1)
	b = binary.LittleEndian.AppendUint32(b, 0xc1000108)
	return append(b, 0)
}

func TestPet434PacketDecoding(t *testing.T) {
	guid := uint64(0xf140000400000004)
	for reaction := byte(0); reaction < 4; reaction++ {
		for command := byte(0); command < 5; command++ {
			b := petPacketFixture434(guid, reaction, command)
			got, err := parsePetSpells434(b)
			if err != nil || got.GUID != guid || got.Reaction != reaction || got.Command != command || len(got.Spells) != 1 {
				t.Fatal(got, err)
			}
			for i := 0; i < len(b); i++ {
				if _, err := parsePetSpells434(b[:i]); err == nil {
					t.Fatalf("accepted truncation %d", i)
				}
			}
			if _, err := parsePetSpells434(append(b, 0)); err == nil {
				t.Fatal("trailing byte")
			}
		}
	}
	for _, pair := range [][2]byte{{4, 0}, {0, 5}} {
		if _, err := parsePetSpells434(petPacketFixture434(guid, pair[0], pair[1])); err == nil {
			t.Fatal("unknown mode accepted")
		}
	}
	if state, err := parsePetSpells434(make([]byte, 8)); err != nil || state.GUID != 0 {
		t.Fatal(state, err)
	}
	b := binary.LittleEndian.AppendUint64(nil, guid)
	b = binary.LittleEndian.AppendUint32(b, 0x11223344)
	mode, err := parsePetMode434(b)
	if err != nil || mode.Raw != 0x11223344 || mode.GUID != guid {
		t.Fatal(mode, err)
	}
}

func TestPet434IdentityAndSafety(t *testing.T) {
	for _, name := range []string{"safe", "unknown", "defensive", "aggressive", "assist", "stay", "attack", "no follow proof", "target", "combat", "dead", "wrong owner", "charmed", "ordinary creature", "different map", "missing object", "extra owned", "packet mismatch"} {
		t.Run(name, func(t *testing.T) {
			c, p, n := petFixture434(t)
			state := &PetSpells434{GUID: n.GUID, Reaction: 0, Command: 1}
			verified := true
			switch name {
			case "unknown":
				state = nil
			case "defensive":
				state.Reaction = 1
			case "aggressive":
				state.Reaction = 2
			case "assist":
				state.Reaction = 3
			case "stay":
				state.Command = 0
			case "attack":
				state.Command = 2
			case "no follow proof":
				verified = false
			case "target":
				n.Fields[combatTarget434] = 7
			case "combat":
				n.Fields[combatFlags434] |= 0x80000
			case "dead":
				n.Fields[FieldHealth434] = 0
			case "wrong owner":
				n.Fields[0x10] = p.Fields[0x10] + 99
			case "charmed":
				n.Fields[0xe] = 17
			case "ordinary creature":
				n.GUID = 0xf130000400000004
			case "different map":
				n.Map = 2
			case "missing object":
				delete(c.result.Observation.World.Store.objects, n.GUID)
			case "extra owned":
				extra := *n
				extra.GUID++
				c.result.Observation.World.Store.objects[extra.GUID] = &extra
			case "packet mismatch":
				state.GUID++
			}
			got, err := projectPetState434(&c.result.Observation.World.Store, state, verified)
			if got.Safe != (name == "safe") {
				t.Fatal(name, got, err)
			}
			if name == "safe" && (!got.IdentityKnown || got.OwnerGUID != p.GUID || got.Entry != 42721 || got.Unit.Health != 50 || got.Unit.Target != 0) {
				t.Fatal(got)
			}
		})
	}
}

func TestPet434PassiveSerialization(t *testing.T) {
	g := uint64(0xf140000400000004)
	for _, follow := range []bool{false, true} {
		b, err := safePetAction434(g, follow)
		if err != nil || len(b) != 32 || binary.LittleEndian.Uint64(b) != g {
			t.Fatal(b, err)
		}
		expected := uint32(0x06000000)
		if follow {
			expected = 0x07000001
		}
		if binary.LittleEndian.Uint32(b[8:]) != expected || !reflect.DeepEqual(b[12:], make([]byte, 20)) {
			t.Fatal(b)
		}
	}
	if _, err := safePetAction434(0, false); err == nil {
		t.Fatal("zero guid accepted")
	}
}

func TestPet434UnknownDoesNotSendControlOrCombat(t *testing.T) {
	c, _, _ := petFixture434(t)
	var ops []uint16
	send := func(op uint16, b []byte) error { ops = append(ops, op); return nil }
	if err := c.tick(1000, send); err != nil {
		t.Fatal(err)
	}
	if err := c.tick(5999, send); err != nil {
		t.Fatal(err)
	}
	err := c.tick(6000, send)
	if err == nil || !strings.Contains(err.Error(), "UNKNOWN") || c.result.SafetyProven || c.result.ControlSent || !reflect.DeepEqual(ops, []uint16{0x4924}) {
		t.Fatal(err, ops, c.result)
	}
}

func TestPet434ControlRequiresAuthoritativeTransition(t *testing.T) {
	for _, initial := range []byte{0, 1, 2, 3} {
		t.Run(string(rune('0'+initial)), func(t *testing.T) {
			c, _, pet := petFixture434(t)
			var ops []uint16
			send := func(op uint16, b []byte) error { ops = append(ops, op); return nil }
			if err := c.tick(1000, send); err != nil {
				t.Fatal(err)
			}
			if err := c.observe(loginPacket434{op: 0x4114, body: petPacketFixture434(pet.GUID, initial, 1)}); err != nil {
				t.Fatal(err)
			}
			if err := c.tick(1100, send); err != nil {
				t.Fatal(err)
			}
			if c.result.SafetyProven || c.result.Pet.Safe || !c.result.ControlSent {
				t.Fatal("commanded state treated as observed")
			}
			expected := []uint16{0x4924, 0x0226, 0x4924}
			if initial != 0 {
				expected = []uint16{0x4924, 0x0226, 0x0226, 0x4924}
			}
			if !reflect.DeepEqual(ops, expected) {
				t.Fatal(ops)
			}
			if err := c.observe(loginPacket434{op: 0x4114, body: petPacketFixture434(pet.GUID, 0, 1)}); err != nil {
				t.Fatal(err)
			}
			if err := c.tick(1200, send); err != nil {
				t.Fatal(err)
			}
			if !c.result.SafetyProven || !c.result.Pet.Safe || !c.done {
				t.Fatal(c.result)
			}
		})
	}
}

func TestPet434UnverifiedPetCannotStartObjectiveCombat(t *testing.T) {
	for _, reaction := range []byte{0, 1, 2, 3} {
		c, p, _ := combatFixture434(t)
		p.Fields[0xa] = 4
		p.Fields[0xb] = 0xf1400004
		var sent []uint16
		err := c.tick(1000, func(op uint16, b []byte) error { sent = append(sent, op); return nil })
		if err == nil || len(sent) != 0 {
			t.Fatal(reaction, err, sent)
		}
	}
}
