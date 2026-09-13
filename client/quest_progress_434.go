package client

import (
	"encoding/binary"
	"fmt"
)

const (
	cataQuestQueryInfo        = 0x0d06
	cataQuestQueryResponse    = 0x6936
	cataQuestCredit           = 0x0d27
	cataQuestPVPCredit        = 0x4416
	cataQuestProgressComplete = 0x2937
	cataQuestTimerFailed      = 0x6427
	cataQuestGiverFailed      = 0x4236
)

type QuestRequirement434 struct {
	Index            int
	Kind             string
	Target, Required uint32
	Text             string
}

// Definition is immutable protocol data, separate from the live object fields.
// Prefix retains every fixed word, including unmodeled rewards/metadata and float bits.
type QuestDefinition434 struct {
	ID                                                uint32
	Type, Level, MinLevel                             int32
	Flags                                             uint32
	Title, Summary, Description, Area, CompletionText string
	Prefix                                            [70]uint32
	Targets                                           [4][4]uint32 // encoded NPC/GO, count, item-drop ID, quantity
	Items                                             [6][2]uint32
	RequiredSpell                                     uint32
	ObjectiveText                                     [4]string
	RewardCurrency, RequiredCurrency                  [4][2]uint32
	PortraitText                                      [4]string
	Sound                                             [2]uint32
}

// QueryQuestInfoResponse::Write, TCPP 15595. No packed GUID or variable arrays.
func parseQuestDefinition434(b []byte) (QuestDefinition434, error) {
	r := newObjectReader434(b)
	var d QuestDefinition434
	for i := range d.Prefix {
		d.Prefix[i] = r.u32()
	}
	d.ID = d.Prefix[0]
	d.Type = int32(d.Prefix[1])
	d.Level = int32(d.Prefix[2])
	d.MinLevel = int32(d.Prefix[3])
	d.Flags = d.Prefix[20]
	d.Title = gossipString434(r)
	d.Summary = gossipString434(r)
	d.Description = gossipString434(r)
	d.Area = gossipString434(r)
	d.CompletionText = gossipString434(r)
	for i := range d.Targets {
		for j := range d.Targets[i] {
			d.Targets[i][j] = r.u32()
		}
	}
	for i := range d.Items {
		for j := range d.Items[i] {
			d.Items[i][j] = r.u32()
		}
	}
	d.RequiredSpell = r.u32()
	for i := range d.ObjectiveText {
		d.ObjectiveText[i] = gossipString434(r)
	}
	for i := range d.RewardCurrency {
		for j := range d.RewardCurrency[i] {
			d.RewardCurrency[i][j] = r.u32()
		}
	}
	for i := range d.RequiredCurrency {
		for j := range d.RequiredCurrency[i] {
			d.RequiredCurrency[i][j] = r.u32()
		}
	}
	for i := range d.PortraitText {
		d.PortraitText[i] = gossipString434(r)
	}
	for i := range d.Sound {
		d.Sound[i] = r.u32()
	}
	if err := r.done(); err != nil {
		return d, err
	}
	if d.ID == 0 || d.ID&0x80000000 != 0 {
		return d, fmt.Errorf("invalid/unsupported quest definition ID")
	}
	return d, nil
}

func (d QuestDefinition434) Requirements() []QuestRequirement434 {
	var out []QuestRequirement434
	for i, t := range d.Targets {
		if t[0] == 0 && t[1] == 0 {
			continue
		}
		kind := "creature-credit"
		if t[0]&0x80000000 != 0 {
			kind = "gameobject-credit"
		}
		if t[0] == 0 {
			kind = "unknown"
		}
		out = append(out, QuestRequirement434{i, kind, t[0] & 0x7fffffff, t[1], d.ObjectiveText[i]})
	}
	for i, t := range d.Items {
		if t[0] != 0 || t[1] != 0 {
			out = append(out, QuestRequirement434{i, "item", t[0], t[1], ""})
		}
	}
	for i, t := range d.RequiredCurrency {
		if t[0] != 0 || t[1] != 0 {
			out = append(out, QuestRequirement434{i, "currency", t[0], t[1], ""})
		}
	}
	if d.Prefix[23] != 0 {
		out = append(out, QuestRequirement434{0, "player-credit", 0, d.Prefix[23], ""})
	}
	if d.Flags&6 != 0 {
		out = append(out, QuestRequirement434{0, "event-or-exploration", 0, 1, d.Area})
	}
	if d.RequiredSpell != 0 {
		out = append(out, QuestRequirement434{0, "required-spell", d.RequiredSpell, 1, ""})
	}
	if len(out) == 0 {
		out = append(out, QuestRequirement434{0, "unknown", 0, 0, d.Summary})
	}
	return out
}

type QuestProgressEvent434 struct {
	Opcode                                   uint16
	QuestID, Target, Count, Required, Reason uint32
	VictimGUID                               uint64
}

// Notifications are evidence, not authority to invent or increment slot counters.
// 0x2937 is also sent for exploration credit before CanCompleteQuest succeeds.
func parseQuestProgressEvent434(op uint16, b []byte) (QuestProgressEvent434, bool, error) {
	e := QuestProgressEvent434{Opcode: op}
	r := newObjectReader434(b)
	switch op {
	case cataQuestCredit:
		e.QuestID = r.u32()
		e.Target = r.u32()
		e.Count = r.u32()
		e.Required = r.u32()
		e.VictimGUID = r.u64()
	case cataQuestPVPCredit:
		e.QuestID = r.u32()
		e.Count = r.u32()
		e.Required = r.u32()
	case cataQuestProgressComplete, cataQuestTimerFailed:
		e.QuestID = r.u32()
	case cataQuestGiverFailed:
		e.QuestID = r.u32()
		e.Reason = r.u32()
	default:
		return e, false, nil // unused 0x6324 / unimplemented item opcode have no guessed decoder
	}
	if err := r.done(); err != nil {
		return e, true, err
	}
	if e.QuestID == 0 {
		return e, true, fmt.Errorf("zero quest ID in progress notification")
	}
	return e, true, nil
}

type QuestObjectiveProgress434 struct {
	QuestRequirement434
	Current         uint32
	Known, Complete bool
	Source          string
}
type QuestProgress434 struct {
	Definition QuestDefinition434
	Slot       QuestSlot434
	Status     string
	Objectives []QuestObjectiveProgress434
}

func ProjectQuestProgress434(d QuestDefinition434, s *ObjectStore434) (QuestProgress434, error) {
	out := QuestProgress434{Definition: d}
	slots, err := QuestLog434(s)
	if err != nil {
		return out, err
	}
	found := false
	for _, q := range slots {
		if q.ID == d.ID {
			out.Slot = q
			found = true
			break
		}
	}
	if !found {
		return out, fmt.Errorf("quest %d not active in player fields", d.ID)
	}
	out.Status = "incomplete"
	if out.Slot.State&1 != 0 {
		out.Status = "complete"
	}
	if out.Slot.State&2 != 0 {
		out.Status = "failed"
	}
	if out.Slot.State&^uint32(3) != 0 || out.Slot.State&3 == 3 {
		out.Status = "unknown"
	}
	for _, q := range d.Requirements() {
		p := QuestObjectiveProgress434{QuestRequirement434: q, Source: "unavailable"}
		switch q.Kind {
		case "creature-credit", "gameobject-credit", "player-credit":
			p.Current = uint32(out.Slot.Counters[q.Index])
			p.Known = true
			p.Source = "quest-log fields"
		case "item":
			p.Current, p.Known = questInventoryCount434(s, q.Target)
			p.Source = "server inventory/bank fields"
		case "event-or-exploration":
			// The individual explored bit is server-only. Complete quest implies it,
			// but incomplete quest does not prove the event has not occurred.
			if out.Status == "complete" {
				p.Current = 1
				p.Known = true
				p.Source = "complete quest state"
			}
		}
		p.Complete = p.Known && q.Required > 0 && p.Current >= q.Required
		out.Objectives = append(out.Objectives, p)
	}
	return out, nil
}

// GetItemCount(entry,true) includes equipment, backpack, bags and bank. Missing
// referenced item objects make the result unknown, rather than a false zero.
func questInventoryCount434(s *ObjectStore434, entry uint32) (uint32, bool) {
	player := s.objects[s.PlayerGUID]
	if player == nil || !player.Created {
		return 0, false
	}
	seen := map[uint64]bool{}
	var count uint64
	var visit func(uint64) bool
	visit = func(g uint64) bool {
		if g == 0 {
			return true
		}
		if seen[g] {
			return false
		}
		seen[g] = true
		o := s.objects[g]
		if o == nil || !o.Created || (o.Type != 1 && o.Type != 2) {
			return false
		}
		owner := uint64(o.Fields[8]) | uint64(o.Fields[9])<<32
		if owner != s.PlayerGUID {
			return false
		}
		if o.Fields[FieldEntry434] == entry {
			count += uint64(o.Fields[0x10])
		}
		if o.Type == 2 {
			n := o.Fields[0x4a]
			if n > 36 {
				return false
			}
			for i := uint16(0); i < uint16(n); i++ {
				k := uint16(0x4c) + 2*i
				if !visit(uint64(o.Fields[k]) | uint64(o.Fields[k+1])<<32) {
					return false
				}
			}
		}
		return count <= 0xffffffff
	}
	// 0x92 + 0x12e: equipment/bags, backpack, bank, bank bags (74 GUIDs).
	for i := uint16(0); i < 74; i++ {
		k := uint16(0x1c0) + 2*i
		if !visit(uint64(player.Fields[k]) | uint64(player.Fields[k+1])<<32) {
			return 0, false
		}
	}
	return uint32(count), true
}

func questInfoRequest434(id uint32) []byte { return binary.LittleEndian.AppendUint32(nil, id) }
