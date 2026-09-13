package client

import (
	"context"
	"fmt"
	"github.com/azerothcore/AzerothGhost/pathfinding"
	"strings"
	"time"
)

type NPCApproach434 struct {
	Navigation NavigationAttempt434
	NPC        NPCCandidate434
}
type NPCInteraction434 struct {
	World    WorldState434Result
	NPC      NPCCandidate434
	Gossip   Gossip434
	Target   uint64
	Distance float64
	Sent     []uint16
}

func ApproachNPC434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, finder RouteFinder434, factions *NPCFactions434) (NPCApproach434, error) {
	return approachNPC434(ctx, user, key, realm, name, instance, finder, factions, nil)
}
func approachNPC434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, finder RouteFinder434, factions *NPCFactions434, choose func(*WorldState434Result, []reputation434) (NPCCandidate434, Position434, error)) (NPCApproach434, error) {
	a := NPCApproach434{Navigation: NavigationAttempt434{World: WorldState434Result{Opcodes: map[uint16]int{}, WorldVariables: map[uint32]int32{}}}}
	if finder == nil || factions == nil || name == "" || instance == "" {
		return a, fmt.Errorf("explicit NPC approach inputs required")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	err := withWorld434(ctx, user, key, realm, func(w *worldWire434) error {
		roster, err := enumerateOnWorld434(w)
		if err != nil {
			return err
		}
		character, err := selectLoginCharacter434(roster, name)
		if err != nil {
			return err
		}
		world := &a.Navigation.World
		world.Login.Character = character
		world.Store.PlayerGUID = character.GUID
		if err := w.send(cataPlayerLogin, loginGUID434(character.GUID)); err != nil {
			return err
		}
		var rep []reputation434
		observer := func(p loginPacket434) error {
			if p.op == cataInitializeFactions {
				var err error
				rep, err = parseReputations434(p.body)
				if err != nil {
					return err
				}
			}
			return world.observe(p)
		}
		controller := navigationController434{ctx: ctx, result: &a.Navigation, finder: finder}
		controller.destination = func(s *WorldState434Result) (pathfinding.Point3D, error) {
			if choose != nil {
				var dest Position434
				var err error
				a.NPC, dest, err = choose(s, rep)
				return point434(dest), err
			}
			candidates := npcCandidates434(&s.Store, factions, rep)
			if len(candidates) == 0 {
				return pathfinding.Point3D{}, fmt.Errorf("no proven friendly stationary gossip NPC within 40 units")
			}
			a.NPC = candidates[0]
			p := s.Store.objects[s.Store.PlayerGUID]
			dest, err := npcDestination434(*p.Position, a.NPC)
			return point434(dest), err
		}
		controller.validate = func(s, d pathfinding.Point3D, r *pathfinding.PathResult) ([]Position434, error) {
			return validateBoundedRoute434(s, d, r, 40, 1, 50, 64)
		}
		return awaitSession434(ctx, w, strings.ToUpper(user), instance, &world.Login, openInstance434, observer, func() (bool, error) { return controller.stage == 6, nil }, controller.tick)
	})
	return a, err
}

// Reconnect before Hello: the fresh player snapshot must independently prove
// the approach, and the same living friendly NPC must still be in range.
func InteractNPC434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, factions *NPCFactions434, approach NPCApproach434) (NPCInteraction434, error) {
	return interactNPC434(ctx, user, key, realm, name, instance, factions, approach, nil)
}

type npcConversation434 interface {
	observe(loginPacket434, *NPCInteraction434) error
	tick(uint32, func(uint16, []byte) error, *NPCInteraction434) (bool, error)
}

func interactNPC434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, factions *NPCFactions434, approach NPCApproach434, conversation npcConversation434) (NPCInteraction434, error) {
	a := NPCInteraction434{World: WorldState434Result{Opcodes: map[uint16]int{}, WorldVariables: map[uint32]int32{}}}
	if factions == nil || name == "" || instance == "" {
		return a, fmt.Errorf("explicit NPC interaction inputs required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := withWorld434(ctx, user, key, realm, func(w *worldWire434) error {
		roster, err := enumerateOnWorld434(w)
		if err != nil {
			return err
		}
		character, err := selectLoginCharacter434(roster, name)
		if err != nil {
			return err
		}
		a.World.Login.Character = character
		a.World.Store.PlayerGUID = character.GUID
		if err := w.send(cataPlayerLogin, loginGUID434(character.GUID)); err != nil {
			return err
		}
		var rep []reputation434
		var readyAt, helloAt uint32
		received, closed := false, false
		observer := func(p loginPacket434) error {
			if p.op == cataInitializeFactions {
				var err error
				rep, err = parseReputations434(p.body)
				if err != nil {
					return err
				}
			}
			if p.op == cataGossipMessage {
				if helloAt == 0 || received {
					return fmt.Errorf("unexpected gossip response")
				}
				var err error
				a.Gossip, err = parseGossip434(p.body, a.NPC.GUID)
				if err != nil {
					return err
				}
				received = true
			}
			if p.op == cataGossipComplete && !received {
				return fmt.Errorf("gossip closed without proving intended NPC response")
			}
			if err := a.World.observe(p); err != nil {
				return err
			}
			if conversation != nil {
				return conversation.observe(p, &a)
			}
			return nil
		}
		tick := func(now uint32, send func(uint16, []byte) error) error {
			if closed {
				return nil
			}
			if received { // No client gossip-close opcode exists in this build. Clear target and disconnect.
				if conversation != nil {
					done, err := conversation.tick(now, send, &a)
					if err != nil {
						return err
					}
					if !done {
						return nil
					}
				}
				if err := send(cataSetSelection, targetGUID434(0)); err != nil {
					return err
				}
				a.Target = 0
				a.Sent = append(a.Sent, cataSetSelection)
				closed = true
				return nil
			}
			if helloAt != 0 {
				if now-helloAt > 5000 {
					return fmt.Errorf("gossip response timeout (server silently rejects invalid/range requests)")
				}
				return nil
			}
			ready, err := a.World.playerReady()
			if err != nil {
				return err
			}
			if !ready {
				return nil
			}
			if readyAt == 0 {
				readyAt = now
				return nil
			}
			if now-readyAt < 1000 {
				return nil
			}
			if _, _, err := ValidateNavigationProof434(approach.Navigation, a.World); err != nil {
				return err
			}
			p := a.World.Store.objects[a.World.Store.PlayerGUID]
			npc := a.World.Store.objects[approach.NPC.GUID]
			if npc == nil || npc.Type != 3 || npc.Position == nil || npc.Fields[FieldHealth434] == 0 || npc.Fields[FieldNPCFlags434]&1 == 0 || npc.Fields[FieldEntry434] != approach.NPC.Entry || !factions.friendly(*npc, *p, rep) {
				return fmt.Errorf("selected NPC no longer valid/friendly")
			}
			a.Distance = distance434(point434(*p.Position), point434(*npc.Position))
			if a.Distance > 3.5 {
				return fmt.Errorf("NPC outside conservative interaction range: %.3f", a.Distance)
			}
			a.NPC = approach.NPC
			a.NPC.Position = *npc.Position
			// These logged-in requests are accepted by WorldSession; the server sends
			// GossipMessage on the realm connection, observed by the same session loop.
			if err := send(cataSetSelection, targetGUID434(a.NPC.GUID)); err != nil {
				return err
			}
			a.Target = a.NPC.GUID
			a.Sent = append(a.Sent, cataSetSelection)
			if err := send(cataGossipHello, targetGUID434(a.NPC.GUID)); err != nil {
				return err
			}
			a.Sent = append(a.Sent, cataGossipHello)
			helloAt = now
			return nil
		}
		return awaitSession434(ctx, w, strings.ToUpper(user), instance, &a.World.Login, openInstance434, observer, func() (bool, error) { return closed, nil }, tick)
	})
	return a, err
}
