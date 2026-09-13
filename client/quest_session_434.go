package client

import (
	"context"
	"fmt"
	"math"
)

// ApproachQuestNPC434 reuses the NPC movement executor and its MMap validator.
// preferredEntry is a probe preference; identities always come from live state.
func ApproachQuestNPC434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, finder RouteFinder434, factions *NPCFactions434, preferredEntry uint32) (NPCApproach434, error) {
	return approachNPC434(ctx, user, key, realm, name, instance, finder, factions, func(s *WorldState434Result, rep []reputation434) (NPCCandidate434, Position434, error) {
		candidates := npcCandidatesWithin434(&s.Store, factions, rep, 0)
		var selected *NPCCandidate434
		for i := range candidates {
			n := &candidates[i]
			if n.Flags&2 == 0 {
				continue
			}
			if selected == nil {
				selected = n
			}
			if preferredEntry != 0 && n.Entry == preferredEntry {
				selected = n
				break
			}
		}
		if selected == nil {
			return NPCCandidate434{}, Position434{}, fmt.Errorf("no live friendly questgiver")
		}
		p := s.Store.objects[s.Store.PlayerGUID].Position
		d, err := safeQuestApproach434(ctx, finder, uint32(s.Login.Map), *p, *selected)
		return *selected, d, err
	})
}

// Keep spacing while trying deterministic navmesh destinations within the
// minimum TCPP interaction radius (4, before adding NPC combat reach).
func safeQuestApproach434(ctx context.Context, finder RouteFinder434, mapID uint32, player Position434, npc NPCCandidate434) (Position434, error) {
	base := math.Atan2(float64(player.Y-npc.Position.Y), float64(player.X-npc.Position.X))
	var lastErr error
	for _, offset := range []float64{0.65, -0.65, 1.3, -1.3, 1.95, -1.95, math.Pi} {
		if err := ctx.Err(); err != nil {
			return Position434{}, err
		}
		d := Position434{X: npc.Position.X + float32(2.5*math.Cos(base+offset)), Y: npc.Position.Y + float32(2.5*math.Sin(base+offset)), Z: npc.Position.Z}
		r, err := finder.FindPath(mapID, point434(player), point434(d))
		if err == nil {
			_, err = validateBoundedRoute434(point434(player), point434(d), r, 40, 1, 50, 64)
		}
		if err == nil && distance434(r.Points[len(r.Points)-1], point434(npc.Position)) > 3.5 {
			err = fmt.Errorf("projected approach outside conservative interaction radius")
		}
		if err == nil {
			return d, nil
		}
		lastErr = err
	}
	return Position434{}, fmt.Errorf("no safe navmesh interaction destination: %w", lastErr)
}

type QuestAcceptance434 struct {
	Interaction       NPCInteraction434
	Selected          GossipQuest434
	Details           QuestDetails434
	InitialLog        []QuestSlot434
	Accepted          *QuestSlot434 // only populated by the server's object fields
	AcceptSent        bool
	AcquisitionMethod QuestAcquisitionMethod434
	Sent              []uint16
}
type questConversation434 struct {
	result       *QuestAcceptance434
	approvedID   uint32 // optional inspected-ID gate; AUTO_ACCEPT offers use direct acceptance
	stage        int
	since        uint32
	detailsReady bool
}

func (q *questConversation434) observe(p loginPacket434, a *NPCInteraction434) error {
	switch p.op {
	case cataQuestDetails:
		if q.stage != 1 || q.detailsReady {
			return fmt.Errorf("unexpected quest details")
		}
		d, err := parseQuestDetails434(p.body, a.NPC.GUID, q.result.Selected.ID)
		if err != nil {
			return err
		}
		q.result.Details = d
		q.detailsReady = true
	case cataQuestInvalid:
		r := newObjectReader434(p.body)
		reason := r.u32()
		if err := r.done(); err != nil {
			return err
		}
		return fmt.Errorf("server rejected quest: reason %d", reason)
	case cataQuestLogFull:
		if len(p.body) != 0 {
			return fmt.Errorf("malformed quest-log-full packet")
		}
		return fmt.Errorf("server reports quest log full")
	}
	return nil
}
func (q *questConversation434) tick(now uint32, send func(uint16, []byte) error, a *NPCInteraction434) (bool, error) {
	r := q.result
	if q.stage == 0 {
		log, err := QuestLog434(&a.World.Store)
		if err != nil {
			return false, err
		}
		r.InitialLog = append([]QuestSlot434(nil), log...)
		selected, err := selectQuest434(a.Gossip, log, a.World.Store.objects[a.World.Store.PlayerGUID].Fields[FieldLevel434])
		if err != nil {
			return false, err
		}
		if q.approvedID != 0 && selected.ID != q.approvedID {
			return false, fmt.Errorf("rediscovered quest differs from inspected quest; no acceptance sent")
		}
		r.Selected = selected
		// TCPP accepts directly from the live giver/quest pair; opening details
		// is not a prerequisite. Querying AUTO_ACCEPT details would add the
		// quest before our explicit request, so send the accept directly.
		if selected.Flags&0x80000 != 0 {
			if err := send(cataQuestAccept, questAccept434(a.NPC.GUID, selected.ID)); err != nil {
				return false, err
			}
			r.AcceptSent = true
			r.AcquisitionMethod = QuestAcquisitionExplicitAccept434
			r.Sent = append(r.Sent, cataQuestAccept)
			q.stage = 2
			q.since = now
			return false, nil
		}
		if err := send(cataQuestDetailsRequest, questDetailsRequest434(a.NPC.GUID, selected.ID)); err != nil {
			return false, err
		}
		r.Sent = append(r.Sent, cataQuestDetailsRequest)
		q.stage = 1
		q.since = now
		return false, nil
	}
	if now-q.since > 8000 {
		return false, fmt.Errorf("quest stage %d timeout; accept sent=%t", q.stage, r.AcceptSent)
	}
	if q.stage == 1 && q.detailsReady {
		d := r.Details
		if d.Flags != r.Selected.Flags || d.InformUnit != 0 || d.StartCheat || d.RequiredSpell != 0 || d.SuggestedParty > 1 {
			return false, fmt.Errorf("quest details outside conservative acceptance scope")
		}
		log, err := QuestLog434(&a.World.Store)
		if err != nil {
			return false, err
		}
		for _, slot := range log {
			if slot.ID == r.Selected.ID {
				return false, fmt.Errorf("quest entered log before explicit acceptance")
			}
		}
		if d.Flags&0x80000 != 0 {
			r.AcquisitionMethod = QuestAcquisitionAutoAccept434
			q.stage = 2
			q.since = now
			return false, nil
		}
		if q.approvedID == 0 {
			q.stage = 3
			return true, nil
		}
		if err := send(cataQuestAccept, questAccept434(a.NPC.GUID, d.ID)); err != nil {
			return false, err
		}
		r.AcceptSent = true
		r.Sent = append(r.Sent, cataQuestAccept)
		q.stage = 2
		q.since = now
		return false, nil
	}
	if q.stage == 2 {
		log, err := QuestLog434(&a.World.Store)
		if err != nil {
			return false, err
		}
		for _, slot := range log {
			if slot.ID == r.Selected.ID {
				if len(log) != len(r.InitialLog)+1 {
					return false, fmt.Errorf("unexpected quest-log additions")
				}
				for _, old := range r.InitialLog {
					found := false
					for _, current := range log {
						if current == old {
							found = true
						}
					}
					if !found {
						return false, fmt.Errorf("preexisting quest state changed")
					}
				}
				copySlot := slot
				r.Accepted = &copySlot
				if r.AcquisitionMethod == QuestAcquisitionUnknown434 {
					r.AcquisitionMethod = QuestAcquisitionExplicitAccept434
				}
				q.stage = 3
				return true, nil
			}
		}
	}
	return q.stage == 3, nil
}

// An inspected ID optionally gates acceptance. AUTO_ACCEPT offers are accepted
// directly because a details query would itself acquire them. All IDs come from gossip.
func QuestConversation434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, factions *NPCFactions434, approach NPCApproach434, inspectedQuestID uint32) (QuestAcceptance434, error) {
	r := QuestAcceptance434{}
	c := questConversation434{result: &r, approvedID: inspectedQuestID}
	a, err := interactNPC434(ctx, user, key, realm, name, instance, factions, approach, &c)
	r.Interaction = a
	return r, err
}
func VerifyQuestReconnect434(r QuestAcceptance434, s WorldState434Result) (QuestSlot434, error) {
	if !r.AcceptSent || r.Accepted == nil || s.Store.PlayerGUID != r.Interaction.World.Store.PlayerGUID || s.Login.Character.GUID != r.Interaction.World.Login.Character.GUID || s.Login.Map != r.Interaction.World.Login.Map {
		return QuestSlot434{}, fmt.Errorf("quest reconnect identity/acceptance mismatch")
	}
	if ready, err := s.playerReady(); err != nil || !ready {
		return QuestSlot434{}, fmt.Errorf("quest reconnect player not ready: %v", err)
	}
	log, err := QuestLog434(&s.Store)
	if err != nil {
		return QuestSlot434{}, err
	}
	for _, slot := range log {
		if slot.ID == r.Selected.ID {
			if slot.State != r.Accepted.State || slot.Counters != r.Accepted.Counters || slot.Timer != r.Accepted.Timer {
				return slot, fmt.Errorf("quest state changed across reconnect")
			}
			return slot, nil
		}
	}
	return QuestSlot434{}, fmt.Errorf("accepted quest absent from fresh server snapshot")
}
