package client

import (
	"fmt"
	"math"
	"sort"
)

const (
	cataAttackSwing434      = 0x0926
	cataAttackStop434       = 0x4106
	cataAttackStart434      = 0x2d15
	cataServerAttackStop434 = 0x0934
	combatFlags434          = 0x35
	combatDynamic434        = 0x49
	combatTarget434         = 0x14
	combatReach434          = 0x3c
	combatForbidden434      = 0x2 | 0x80 | 0x100 | 0x10000 | 0x100000 | 0x2000000
)

// This deliberately bounded adapter accepts one plain creature objective only.
// The entry is derived from the queried definition, never from a chosen GUID.
func singleObjective434(q QuestProgress434, count uint32) (QuestObjectiveProgress434, error) {
	if q.Status != "incomplete" || q.Slot.State != 0 || q.Slot.Timer != 0 || len(q.Objectives) != 1 {
		return QuestObjectiveProgress434{}, fmt.Errorf("BAD_TEST: need one active untimed incomplete objective")
	}
	o := q.Objectives[0]
	if o.Kind != "creature-credit" || !o.Known || o.Target == 0 || o.Required != 6 || o.Current != count || q.Definition.RequiredSpell != 0 {
		return o, fmt.Errorf("BAD_TEST: expected plain creature objective %d/6, observed %+v", count, o)
	}
	return o, nil
}

type CombatUnit434 struct {
	GUID                                               uint64
	Health, MaxHealth, Power, MaxPower, Flags, Dynamic uint32
	Target                                             uint64
	Dead, InCombat                                     bool
}

func combatUnit434(o *Object434) (CombatUnit434, error) {
	if o == nil || !o.Created || (o.Type != 3 && o.Type != 4) {
		return CombatUnit434{}, fmt.Errorf("COMBAT_STATE_BUG: missing created unit")
	}
	u := CombatUnit434{GUID: o.GUID, Health: o.Fields[FieldHealth434], MaxHealth: o.Fields[FieldMaxHealth434], Flags: o.Fields[combatFlags434], Dynamic: o.Fields[combatDynamic434]}
	if u.MaxHealth == 0 || u.Health > u.MaxHealth {
		return u, fmt.Errorf("COMBAT_STATE_BUG: invalid health")
	}
	u.Target = uint64(o.Fields[combatTarget434]) | uint64(o.Fields[combatTarget434+1])<<32
	u.Dead = u.Health == 0 || u.Dynamic&0x20 != 0
	u.InCombat = u.Flags&0x80000 != 0
	packed := o.Fields[FieldUnitBytes434]
	if slot, ok := PrimaryPowerSlot434(uint8(packed>>8), uint8(packed>>24)); ok {
		u.Power, u.MaxPower = o.Fields[FieldPower1434+slot], o.Fields[FieldMaxPower1434+slot]
	}
	return u, nil
}

// Only ordinary non-reputation NPC factions are supported. Unknown faction,
// reputation and player-controlled targets are rejected rather than guessed.
func (f *NPCFactions434) combatReaction434(n, p *Object434) (string, bool) {
	if f == nil || n == nil || p == nil {
		return "missing faction", false
	}
	a, b := f.templates[n.Fields[FieldFaction434]], f.templates[p.Fields[FieldFaction434]]
	if len(a) < 14 || len(b) < 14 || n.Fields[combatFlags434]&8 != 0 {
		return "unknown/controlled faction", false
	}
	if templateFriendly434(a, b) || templateFriendly434(b, a) {
		return "friendly", false
	}
	faction := f.factions[a[1]]
	if len(faction) < 18 || int32(faction[1]) >= 0 || a[2]&0x1000 != 0 {
		return "reputation/contested faction unsupported", false
	}
	hostile := a[3]&b[5] != 0 || b[3]&a[5] != 0 || a[2]&0x2000 != 0
	for _, v := range a[6:10] {
		hostile = hostile || v != 0 && v == b[1]
	}
	for _, v := range b[6:10] {
		hostile = hostile || v != 0 && v == a[1]
	}
	if hostile {
		return "hostile: faction template enemy/mask", true
	}
	return "neutral attackable: neither friendly, no reputation faction", true
}

type ObjectiveCandidate434 struct {
	GUID                              uint64
	Entry, Faction, Health, MaxHealth uint32
	Position                          Position434
	Distance                          float64
	Reaction                          string
}

func objectiveCandidates434(s *ObjectStore434, f *NPCFactions434, entry uint32) []ObjectiveCandidate434 {
	p := s.objects[s.PlayerGUID]
	if p == nil || p.Position == nil {
		return nil
	}
	var out []ObjectiveCandidate434
	for _, n := range s.Objects() {
		if entry == 0 || n.Fields[FieldEntry434] != entry || n.Type != 3 || n.GUID>>52 != 0xf13 || n.Map != p.Map || n.Position == nil || !finitePosition434(n.Position) {
			continue
		}
		u, err := combatUnit434(&n)
		if err != nil || u.Dead || u.InCombat || u.Target != 0 || u.Health != u.MaxHealth || u.Flags&combatForbidden434 != 0 || u.Dynamic&4 != 0 {
			continue
		}
		if n.Fields[0xe] != 0 || n.Fields[0xf] != 0 || n.Fields[0x10] != 0 || n.Fields[0x11] != 0 {
			continue
		}
		reaction, ok := f.combatReaction434(&n, p)
		if !ok {
			continue
		}
		d := distance434(point434(*p.Position), point434(*n.Position))
		if d < 4 || d > 40 {
			continue
		}
		out = append(out, ObjectiveCandidate434{n.GUID, entry, n.Fields[FieldFaction434], u.Health, u.MaxHealth, *n.Position, d, reaction})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Distance != out[j].Distance {
			return out[i].Distance < out[j].Distance
		}
		return out[i].GUID < out[j].GUID
	})
	return out
}

func meleeRange434(p, n *Object434) (float64, error) {
	if p == nil || n == nil {
		return 0, fmt.Errorf("COMBAT_STATE_BUG: missing range units")
	}
	a, b := float64(math.Float32frombits(p.Fields[combatReach434])), float64(math.Float32frombits(n.Fields[combatReach434]))
	if math.IsNaN(a+b) || math.IsInf(a+b, 0) || a < 0 || b < 0 || a > 10 || b > 10 {
		return 0, fmt.Errorf("COMBAT_STATE_BUG: invalid combat reach")
	}
	return math.Max(5, a+b+1.3333334), nil
}

type CombatEvent434 struct {
	Opcode           uint16
	Attacker, Victim uint64
	NowDead          bool
}

func parseCombatEvent434(op uint16, b []byte) (CombatEvent434, bool, error) {
	e := CombatEvent434{Opcode: op}
	r := newObjectReader434(b)
	switch op {
	case cataAttackStart434:
		e.Attacker = r.u64()
		e.Victim = r.u64()
	case cataServerAttackStop434:
		e.Attacker = r.packed()
		e.Victim = r.packed()
		v := r.u32()
		if v > 1 {
			return e, true, fmt.Errorf("PROTOCOL_HARNESS_BUG: invalid death flag")
		}
		e.NowDead = v == 1
	default:
		return e, false, nil
	}
	return e, true, r.done()
}
