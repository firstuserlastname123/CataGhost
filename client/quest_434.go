package client

import (
	"encoding/binary"
	"fmt"
	"sort"
)

const (
	cataQuestDetailsRequest        = 0x2f14
	cataQuestDetails               = 0x2425
	cataQuestAccept                = 0x6b37
	cataQuestInvalid               = 0x4016
	cataQuestLogFull               = 0x0e36
	questLogBase434         uint16 = 0x9d // UNIT_END (0x92) + 0x0b
	questLogSlots434               = 25   // TCPP MAX_QUEST_LOG_SIZE, not all 50 reserved client slots
)

func questDetailsRequest434(guid uint64, id uint32) []byte {
	b := binary.LittleEndian.AppendUint32(targetGUID434(guid), id)
	return append(b, 0) // bool RespondToGiver
}
func questAccept434(guid uint64, id uint32) []byte {
	b := binary.LittleEndian.AppendUint32(targetGUID434(guid), id)
	return binary.LittleEndian.AppendUint32(b, 0) // uint32 StartCheat, NOT a bool
}

type QuestItem434 struct{ ID, Quantity, Display uint32 }
type QuestRewards434 struct {
	ChoiceCount, ItemCount                       uint32
	Choices                                      [6]QuestItem434
	Items                                        [4]QuestItem434
	Money, XP, Title, BonusTalents, FactionFlags uint32
	FactionID, FactionValue, FactionOverride     [5]uint32
	CompletionDisplaySpell, CompletionSpell      uint32
	CurrencyID, CurrencyQty                      [4]uint32
	Skill, SkillUps                              uint32
}
type QuestDetails434 struct {
	Giver, InformUnit                                                            uint64
	ID                                                                           uint32
	Title, Description, Objectives                                               string
	PortraitGiverText, PortraitGiverName, PortraitTurnInText, PortraitTurnInName string
	PortraitGiver, PortraitTurnIn                                                uint32
	AutoLaunched, StartCheat, DisplayPopup                                       bool
	Flags, SuggestedParty, RequiredSpell                                         uint32
	Rewards                                                                      QuestRewards434
	Emotes                                                                       [][2]uint32
}
type QuestAcquisitionMethod434 uint8

const (
	QuestAcquisitionUnknown434 QuestAcquisitionMethod434 = iota
	QuestAcquisitionAutoAccept434
	QuestAcquisitionExplicitAccept434
)

func questBool434(r *objectReader434) bool {
	v := r.u8()
	if v > 1 {
		r.err = fmt.Errorf("invalid quest boolean")
	}
	return v == 1
}
func parseQuestDetails434(b []byte, guid uint64, id uint32) (QuestDetails434, error) {
	r := newObjectReader434(b)
	d := QuestDetails434{Giver: r.u64(), InformUnit: r.u64(), ID: r.u32()}
	if guid == 0 || id == 0 || d.Giver != guid || d.ID != id {
		return d, fmt.Errorf("quest details giver/ID mismatch")
	}
	d.Title = gossipString434(r)
	d.Description = gossipString434(r)
	d.Objectives = gossipString434(r)
	d.PortraitGiverText = gossipString434(r)
	d.PortraitGiverName = gossipString434(r)
	d.PortraitTurnInText = gossipString434(r)
	d.PortraitTurnInName = gossipString434(r)
	d.PortraitGiver = r.u32()
	d.PortraitTurnIn = r.u32()
	d.AutoLaunched = questBool434(r)
	d.Flags = r.u32()
	d.SuggestedParty = r.u32()
	d.StartCheat = questBool434(r)
	d.DisplayPopup = questBool434(r)
	d.RequiredSpell = r.u32()
	w := &d.Rewards
	w.ChoiceCount = r.u32()
	if w.ChoiceCount > 6 {
		return d, fmt.Errorf("quest reward choices exceed six")
	}
	for i := range w.Choices {
		w.Choices[i].ID = r.u32()
	}
	for i := range w.Choices {
		w.Choices[i].Quantity = r.u32()
	}
	for i := range w.Choices {
		w.Choices[i].Display = r.u32()
	}
	w.ItemCount = r.u32()
	if w.ItemCount > 4 {
		return d, fmt.Errorf("quest guaranteed rewards exceed four")
	}
	for i := range w.Items {
		w.Items[i].ID = r.u32()
	}
	for i := range w.Items {
		w.Items[i].Quantity = r.u32()
	}
	for i := range w.Items {
		w.Items[i].Display = r.u32()
	}
	w.Money = r.u32()
	w.XP = r.u32()
	w.Title = r.u32()
	r.u32()
	r.u32()
	w.BonusTalents = r.u32()
	r.u32()
	w.FactionFlags = r.u32()
	for i := range w.FactionID {
		w.FactionID[i] = r.u32()
	}
	for i := range w.FactionValue {
		w.FactionValue[i] = r.u32()
	}
	for i := range w.FactionOverride {
		w.FactionOverride[i] = r.u32()
	}
	w.CompletionDisplaySpell = r.u32()
	w.CompletionSpell = r.u32()
	for i := range w.CurrencyID {
		w.CurrencyID[i] = r.u32()
	}
	for i := range w.CurrencyQty {
		w.CurrencyQty[i] = r.u32()
	}
	w.Skill = r.u32()
	w.SkillUps = r.u32()
	n := int(r.u32())
	if n > 4 || !r.bounded(n, 8) {
		return d, fmt.Errorf("invalid quest description emote count")
	}
	for i := 0; i < n; i++ {
		d.Emotes = append(d.Emotes, [2]uint32{r.u32(), r.u32()})
	}
	return d, r.done()
}

// QuestLog434 projects server-provided fields. A created object starts with
// zero fields on this protocol; sparse create masks omit those zeros.
type QuestSlot434 struct {
	Slot      int
	ID, State uint32
	Counters  [4]uint16
	Timer     uint32
}

func QuestLog434(s *ObjectStore434) ([]QuestSlot434, error) {
	p := s.objects[s.PlayerGUID]
	if p == nil || !p.Created || p.Type != 4 {
		return nil, fmt.Errorf("no created player quest snapshot")
	}
	var result []QuestSlot434
	seen := map[uint32]bool{}
	for i := 0; i < questLogSlots434; i++ {
		base := questLogBase434 + uint16(i*5)
		id := p.Fields[base]
		if id == 0 {
			continue
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate quest ID in log")
		}
		seen[id] = true
		lo, hi := p.Fields[base+2], p.Fields[base+3]
		result = append(result, QuestSlot434{i, id, p.Fields[base+1], [4]uint16{uint16(lo), uint16(lo >> 16), uint16(hi), uint16(hi >> 16)}, p.Fields[base+4]})
	}
	return result, nil
}

// Conservative flags: only ordinary, non-recurring quests without scripted
// acceptance/phase/PvP/autocomplete flags qualify for this first probe.
func ordinaryQuestFlags434(flags uint32) bool {
	return flags & ^uint32(0x8|0x80|0x100|0x200|0x800|0x40000|0x04000000|0x08000000|0x10000000|0x40000000) == 0
}
func selectQuest434(g Gossip434, log []QuestSlot434, level uint32) (GossipQuest434, error) {
	present := map[uint32]bool{}
	for _, q := range log {
		present[q.ID] = true
	}
	if len(log) >= questLogSlots434 {
		return GossipQuest434{}, fmt.Errorf("quest log full")
	}
	var available []GossipQuest434
	for _, q := range g.Quests {
		if q.ID != 0 && q.Type == 2 && !q.Repeatable && !present[q.ID] && int32(q.Level) >= -1 && int32(q.Level) <= int32(level)+2 && ordinaryQuestFlags434(q.Flags&^0x80000) {
			available = append(available, q)
		}
	}
	sort.Slice(available, func(i, j int) bool { return available[i].ID < available[j].ID })
	if len(available) == 0 {
		return GossipQuest434{}, fmt.Errorf("no suitable unaccepted live quest")
	}
	return available[0], nil
}
