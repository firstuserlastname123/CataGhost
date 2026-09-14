package client

import (
	"encoding/binary"
	"fmt"
)

const (
	cataRequestPetInfo434 = 0x4924
	cataPetSpells434      = 0x4114
	cataPetMode434        = 0x2235
	cataPetAction434      = 0x0226
)

// PetSpells434 is server state, never a reflection of a command we sent.
// TCPP does not expose CharmInfo::_isCommandAttack in this packet.
type PetSpells434 struct {
	GUID              uint64
	Family            uint16
	Duration          uint32
	Reaction, Command uint8
	Flags             uint16
	ActionBar         [10]uint32
	Spells            []uint32
	CooldownCount     uint8
	CooldownData      []byte
}

func parsePetSpells434(b []byte) (PetSpells434, error) {
	r := newObjectReader434(b)
	out := PetSpells434{GUID: r.u64()}
	if out.GUID == 0 {
		return out, r.done()
	} // Clear action bar; not proof of no pet.
	out.Family = r.u16()
	out.Duration = r.u32()
	out.Reaction = r.u8()
	out.Command = r.u8()
	out.Flags = r.u16()
	if out.Reaction > 3 || out.Command > 4 {
		return out, fmt.Errorf("PET_STATE_DECODER_BUG: invalid reaction/command")
	}
	for i := range out.ActionBar {
		out.ActionBar[i] = r.u32()
	}
	n := int(r.u8())
	if !r.bounded(n, 4) {
		return out, fmt.Errorf("PET_STATE_DECODER_BUG: truncated pet spells")
	}
	for i := 0; i < n; i++ {
		out.Spells = append(out.Spells, r.u32())
	}
	out.CooldownCount = r.u8()
	if r.err != nil {
		return out, fmt.Errorf("PET_STATE_DECODER_BUG: %w", r.err)
	}
	// This local writer emits 6 bytes for OnHold and 14 otherwise, without
	// an OnHold discriminator. Preserve the tail; don't invent cooldown values.
	remaining := len(b) - r.bit/8
	minimum := int(out.CooldownCount) * 6
	if remaining < minimum || remaining > int(out.CooldownCount)*14 || (remaining-minimum)%8 != 0 {
		return out, fmt.Errorf("PET_STATE_DECODER_BUG: invalid cooldown tail length")
	}
	out.CooldownData = append([]byte(nil), r.take(remaining)...)
	return out, r.done()
}

type PetMode434 struct {
	GUID uint64
	Raw  uint32
}

func parsePetMode434(b []byte) (PetMode434, error) {
	r := newObjectReader434(b)
	m := PetMode434{r.u64(), r.u32()}
	return m, r.done()
}

type PetState434 struct {
	PlayerGUID, GUID, OwnerGUID, CharmerGUID uint64
	Entry                                    uint32
	Position                                 *Position434
	Unit                                     CombatUnit434
	IdentityKnown, Active, StateKnown, Safe  bool
	Reaction, Command                        string
	Evidence                                 string
	Initialization                           *PetSpells434
}

func fieldGUID434(o *Object434, index uint16) uint64 {
	if o == nil {
		return 0
	}
	return uint64(o.Fields[index]) | uint64(o.Fields[index+1])<<32
}

// The summon field alone can identify a guardian. Require a Pet high GUID and
// the reverse SUMMONEDBY link, not proximity or the character-screen pet model.
func projectPetState434(s *ObjectStore434, init *PetSpells434, followVerified bool) (PetState434, error) {
	out := PetState434{PlayerGUID: s.PlayerGUID, Reaction: "UNKNOWN", Command: "UNKNOWN"}
	p := s.objects[s.PlayerGUID]
	if p == nil || !p.Created || p.Type != 4 {
		return out, fmt.Errorf("PET_STATE_DECODER_BUG: missing player")
	}
	if fieldGUID434(p, 0x8) != 0 || fieldGUID434(p, 0xe) != 0 || fieldGUID434(p, 0x10) != 0 {
		return out, fmt.Errorf("UNKNOWN: charm/owner relationship on player")
	}
	out.GUID = fieldGUID434(p, 0xa)
	for _, o := range s.Objects() {
		if o.GUID == s.PlayerGUID || o.Type != 3 {
			continue
		}
		if (fieldGUID434(&o, 0x10) == p.GUID || fieldGUID434(&o, 0xe) == p.GUID) && o.GUID != out.GUID && o.GUID != fieldGUID434(p, 0xc) {
			return out, fmt.Errorf("UNKNOWN: additional owned/charmed unit %016X", o.GUID)
		}
	}
	if out.GUID == 0 {
		if init != nil && init.GUID != 0 {
			return out, fmt.Errorf("PET_STATE_DECODER_BUG: pet packet contradicts absent summon")
		}
		out.IdentityKnown = true
		out.Safe = true
		out.Evidence = "created player has no summon/charm and no additional visible controlled unit"
		return out, nil
	}
	out.Active = true
	n := s.objects[out.GUID]
	if n == nil || !n.Created || n.Type != 3 || n.GUID>>52 != 0xf14 || n.Map != p.Map {
		return out, fmt.Errorf("UNKNOWN: linked pet missing or not a same-map Pet object")
	}
	out.OwnerGUID = fieldGUID434(n, 0x10)
	out.CharmerGUID = fieldGUID434(n, 0xe)
	out.Entry = n.Fields[FieldEntry434]
	if out.OwnerGUID != p.GUID || out.CharmerGUID != 0 || out.Entry == 0 || n.Fields[combatFlags434]&8 == 0 {
		return out, fmt.Errorf("PET_STATE_DECODER_BUG: pet ownership/type mismatch")
	}
	out.IdentityKnown = true
	if n.Position != nil {
		v := *n.Position
		out.Position = &v
	}
	var err error
	out.Unit, err = combatUnit434(n)
	if err != nil {
		return out, err
	}
	out.Evidence = "player UNIT_FIELD_SUMMON -> Pet GUID; pet UNIT_FIELD_SUMMONEDBY -> player; PLAYER_CONTROLLED"
	if init == nil || init.GUID == 0 {
		return out, nil
	}
	if init.GUID != n.GUID {
		return out, fmt.Errorf("PET_STATE_DECODER_BUG: initialization GUID mismatch")
	}
	if init.Reaction > 3 || init.Command > 4 {
		return out, fmt.Errorf("PET_STATE_DECODER_BUG: invalid projected mode")
	}
	out.Initialization = init
	out.StateKnown = true
	out.Reaction = []string{"PASSIVE", "DEFENSIVE", "AGGRESSIVE", "ASSIST"}[init.Reaction]
	out.Command = []string{"STAY", "FOLLOW", "ATTACK", "ABANDON", "MOVE_TO"}[init.Command]
	// Strict predicate: server-reported PASSIVE/FOLLOW, living, no target/combat,
	// no control flags. Fresh login resets hidden IsCommandAttack; after a change
	// the probe explicitly sends FOLLOW and requires a fresh info response.
	out.Safe = followVerified && init.Reaction == 0 && init.Command == 1 && init.Flags == 0 && !out.Unit.Dead && !out.Unit.InCombat && out.Unit.Target == 0
	return out, nil
}

// This API cannot serialize pet attack, abandon, arbitrary actions or spells.
func safePetAction434(guid uint64, follow bool) ([]byte, error) {
	if guid == 0 || guid>>52 != 0xf14 {
		return nil, fmt.Errorf("PET_CONTROL_BUG: invalid identified pet GUID")
	}
	action := uint32(0x06000000) // ACT_REACTION, REACT_PASSIVE
	if follow {
		action = 0x07000001
	} // ACT_COMMAND, COMMAND_FOLLOW clears IsCommandAttack
	b := binary.LittleEndian.AppendUint64(nil, guid)
	b = binary.LittleEndian.AppendUint32(b, action)
	return append(b, make([]byte, 20)...), nil // target GUID and XYZ all zero
}
