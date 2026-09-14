package client

import "fmt"

// VerifyUncontrolledPlayer434 checks replicated control relationships in a
// created-object snapshot. It does not certify unreplicated server internals.
// TCPP UpdateFields.h: CHARM=0x08, SUMMON=0x0a, CHARMEDBY=0x0e,
// SUMMONEDBY=0x10, CREATEDBY=0x12. Sparse create fields start at zero.
func VerifyUncontrolledPlayer434(s *ObjectStore434) error {
	if s == nil {
		return fmt.Errorf("COMBAT_STATE_BUG: missing object store")
	}
	p := s.objects[s.PlayerGUID]
	if p == nil || !p.Created || p.Type != 4 || !p.ThisIsYou {
		return fmt.Errorf("COMBAT_STATE_BUG: missing created self player")
	}
	guid := func(o *Object434, field uint16) uint64 {
		return uint64(o.Fields[field]) | uint64(o.Fields[field+1])<<32
	}
	for _, field := range []uint16{0x08, 0x0a, 0x0e, 0x10} {
		if v := guid(p, field); v != 0 {
			return fmt.Errorf("BAD_TEST: player control field 0x%X references %016X", field, v)
		}
	}
	for _, o := range s.objects {
		if o.GUID == p.GUID || o.Type < 3 || o.Type > 4 {
			continue
		}
		for _, field := range []uint16{0x0e, 0x10, 0x12} {
			if guid(o, field) == p.GUID {
				return fmt.Errorf("BAD_TEST: unit %016X has control/creation link 0x%X to player", o.GUID, field)
			}
		}
	}
	return nil
}
