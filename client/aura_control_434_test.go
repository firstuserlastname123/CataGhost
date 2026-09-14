package client

import (
	"encoding/binary"
	"testing"
)

func auraFixture434(spell uint32, flags uint16) []byte {
	b := []byte{1, 17, 3}
	b = binary.LittleEndian.AppendUint32(b, spell)
	if spell == 0 {
		return b
	}
	b = binary.LittleEndian.AppendUint16(b, flags)
	b = append(b, 1, 1)
	if flags&8 == 0 {
		b = append(b, 1, 17)
	}
	if flags&0x20 != 0 {
		b = binary.LittleEndian.AppendUint32(b, 9000)
		b = binary.LittleEndian.AppendUint32(b, 4000)
	}
	if flags&0x40 != 0 {
		for i := 0; i < 3; i++ {
			if flags&(1<<i) != 0 {
				b = binary.LittleEndian.AppendUint32(b, uint32(100+i))
			}
		}
	}
	return b
}

func TestPet434AuraWireAndTransitions(t *testing.T) {
	for _, flags := range []uint16{1, 9, 0x29, 0x67, 0x6d} {
		b := auraFixture434(93321, flags)
		p, err := parseAuraPacket434(0x6916, b)
		if err != nil || p.GUID != 17 || len(p.Slots) != 1 || p.Slots[0].Spell != 93321 || p.Slots[0].Flags != flags {
			t.Fatal(p, err)
		}
		for n := 0; n < len(b); n++ {
			if n == 2 {
				continue
			} // complete empty snapshot is legal
			if _, err := parseAuraPacket434(0x6916, b[:n]); err == nil {
				t.Fatalf("truncated aura accepted at %d", n)
			}
		}
	}
	for _, bad := range [][]byte{{0}, {1, 17, 64, 1, 0, 0, 0}, auraFixture434(93321, 0x100), append(auraFixture434(93321, 9), auraFixture434(93321, 9)[2:]...)} {
		if _, err := parseAuraPacket434(0x4707, bad); err == nil {
			t.Fatal("invalid aura accepted", bad)
		}
	}
	var s auraObserver434
	if s.presence(17, 93321, false).State != "UNKNOWN" {
		t.Fatal("missing baseline")
	}
	p, _ := parseAuraPacket434(0x6916, auraFixture434(93321, 9))
	s.apply(p)
	if s.presence(17, 93321, false).State != "PRESENT" {
		t.Fatal("positive not observed")
	}
	if s.presence(18, 93321, false).State != "UNKNOWN" {
		t.Fatal("wrong unit")
	}
	p, _ = parseAuraPacket434(0x4707, auraFixture434(0, 0))
	s.apply(p)
	if s.presence(17, 93321, false).State != "UNKNOWN" {
		t.Fatal("hidden absence guessed from removal")
	}
	if s.presence(17, 93321, true).State != "ABSENT" {
		t.Fatal("visible absence not tracked")
	}
	p, _ = parseAuraPacket434(0x4707, auraFixture434(93321, 9))
	s.apply(p)
	p, _ = parseAuraPacket434(0x6916, []byte{1, 17})
	s.apply(p)
	if s.presence(17, 93321, false).State != "UNKNOWN" {
		t.Fatal("UPDATE_ALL must not prove hidden absence")
	}
}

func TestPet434ControlGateThreeValued(t *testing.T) {
	for _, tc := range []struct{ aura, want string }{{"PRESENT", "true"}, {"ABSENT", "false"}, {"UNKNOWN", "UNKNOWN"}} {
		got := petControlGate434(3, PetState434{IdentityKnown: true, Active: true}, AuraPresence434{Spell: 93321, State: tc.aura})
		if got.CanControlPet != tc.want {
			t.Fatal(got)
		}
	}
}

func TestPet434ReadOnlyAuraProbeCannotSendActions(t *testing.T) {
	c, _, pet := petFixture434(t)
	c.readOnly = true
	sent := []uint16{}
	send := func(op uint16, b []byte) error { sent = append(sent, op); return nil }
	if err := c.tick(1000, send); err != nil {
		t.Fatal(err)
	}
	if err := c.observe(loginPacket434{op: 0x4114, body: petPacketFixture434(pet.GUID, 2, 1)}); err != nil {
		t.Fatal(err)
	}
	if err := c.tick(1100, send); err != nil {
		t.Fatal(err)
	}
	if !c.done || c.result.ControlSent || len(sent) != 1 || sent[0] != 0x4924 || c.result.SafetyProven {
		t.Fatal(c.result, sent)
	}
}
