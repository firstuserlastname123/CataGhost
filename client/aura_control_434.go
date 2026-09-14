package client

import "fmt"

const (
	cataAuraUpdate434    = 0x4707
	cataAuraUpdateAll434 = 0x6916
	hunterControlAura434 = 93321
)

type AuraSlot434 struct {
	Slot                uint8
	Spell               uint32
	Flags               uint16
	Level, Applications uint8
	Caster              uint64
	Duration, Remaining int32
	Points              [3]int32
}
type AuraPacket434 struct {
	GUID  uint64
	All   bool
	Slots []AuraSlot434
}

// SpellPackets.cpp + AuraApplication::BuildUpdatePacket, build 15595.
// The packet ends the list; there is no aura count. Zero spell removes a slot.
func parseAuraPacket434(op uint16, b []byte) (AuraPacket434, error) {
	out := AuraPacket434{All: op == cataAuraUpdateAll434}
	if op != cataAuraUpdate434 && op != cataAuraUpdateAll434 {
		return out, fmt.Errorf("PROTOCOL_HARNESS_BUG: unexpected aura opcode")
	}
	r := newObjectReader434(b)
	out.GUID = r.packed()
	if r.err != nil || out.GUID == 0 {
		return out, fmt.Errorf("PROTOCOL_HARNESS_BUG: invalid aura GUID")
	}
	seen := map[uint8]bool{}
	for r.bit/8 < len(b) && r.err == nil {
		a := AuraSlot434{Slot: r.u8(), Spell: r.u32()}
		if a.Slot >= 64 || seen[a.Slot] || int32(a.Spell) < 0 {
			return out, fmt.Errorf("PROTOCOL_HARNESS_BUG: invalid/duplicate aura slot or spell")
		}
		seen[a.Slot] = true
		if a.Spell != 0 {
			a.Flags = r.u16()
			a.Level = r.u8()
			a.Applications = r.u8()
			if a.Flags&^uint16(0xff) != 0 {
				return out, fmt.Errorf("PROTOCOL_HARNESS_BUG: unknown aura flags")
			}
			if a.Flags&8 == 0 {
				a.Caster = r.packed()
			}
			if a.Flags&0x20 != 0 {
				a.Duration = int32(r.u32())
				a.Remaining = int32(r.u32())
			}
			if a.Flags&0x40 != 0 {
				for i := 0; i < 3; i++ {
					if a.Flags&(1<<i) != 0 {
						a.Points[i] = int32(r.u32())
					}
				}
			}
		}
		out.Slots = append(out.Slots, a)
	}
	return out, r.done()
}

type AuraPresence434 struct {
	Spell           uint32
	State           string
	VisibleSnapshot bool
	Evidence        string
}
type auraObserver434 struct {
	slots     map[uint64]map[uint8]AuraSlot434
	snapshots map[uint64]bool
}

func (s *auraObserver434) apply(p AuraPacket434) {
	if s.slots == nil {
		s.slots = map[uint64]map[uint8]AuraSlot434{}
		s.snapshots = map[uint64]bool{}
	}
	if p.All || s.slots[p.GUID] == nil {
		s.slots[p.GUID] = map[uint8]AuraSlot434{}
	}
	if p.All {
		s.snapshots[p.GUID] = true
	}
	for _, a := range p.Slots {
		if a.Spell == 0 {
			delete(s.slots[p.GUID], a.Slot)
		} else {
			s.slots[p.GUID][a.Slot] = a
		}
	}
}
func (s *auraObserver434) presence(guid uint64, spell uint32, absenceObservable bool) AuraPresence434 {
	out := AuraPresence434{Spell: spell, State: "UNKNOWN", VisibleSnapshot: s.snapshots[guid], Evidence: "no complete server-visible aura baseline"}
	for _, a := range s.slots[guid] {
		if a.Spell == spell {
			out.State = "PRESENT"
			out.Evidence = "server aura slot currently contains spell"
			return out
		}
	}
	if !absenceObservable {
		out.Evidence = "aura can be hidden by Aura::CanBeSentToClient; even UPDATE_ALL cannot prove HasAura false"
		return out
	}
	if out.VisibleSnapshot {
		out.State = "ABSENT"
		out.Evidence = "complete visible snapshot and subsequent deltas for a guaranteed-visible aura"
	}
	return out
}

type PetControlGate434 struct {
	Class         uint8
	PetEquivalent string
	Aura          AuraPresence434
	CanControlPet string
	Evidence      string
}

func petControlGate434(class uint8, pet PetState434, aura AuraPresence434) PetControlGate434 {
	out := PetControlGate434{Class: class, PetEquivalent: "UNKNOWN", Aura: aura, CanControlPet: "UNKNOWN"}
	if pet.IdentityKnown && pet.Active {
		out.PetEquivalent = "owned live Pet object published through UNIT_FIELD_SUMMON; internal GetPet slot not directly transmitted"
	}
	if class != 3 {
		out.Evidence = "this diagnostic is scoped to Hunter CanControlPet()"
		return out
	}
	switch aura.State {
	case "PRESENT":
		out.CanControlPet = "true"
	case "ABSENT":
		out.CanControlPet = "false"
	}
	out.Evidence = "Player::CanControlPet(0): Hunter returns false exactly when HasAura(93321) is false; no pet-health, command or reaction check"
	return out
}
