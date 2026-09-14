package client

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/azerothcore/AzerothGhost/pathfinding"
)

type ObjectiveAttempt434 struct {
	RouteRejections                                                 []string
	PlayerFields                                                    map[uint16]uint32
	Observation                                                     QuestProgressObservation434
	Navigation                                                      NavigationAttempt434
	Candidate                                                       ObjectiveCandidate434
	Candidates                                                      []ObjectiveCandidate434
	KnownSpells                                                     []uint32
	Player, Target                                                  CombatUnit434
	HealthHistory                                                   []CombatUnit434
	CombatEvents                                                    []CombatEvent434
	Sent                                                            []uint16
	CommandedAttack, ObservedAttack, Death, Credit, Stopped, Passed bool
}

func knownSpells434(b []byte) ([]uint32, error) {
	r := newObjectReader434(b)
	if r.u8() > 1 {
		return nil, fmt.Errorf("invalid known-spells initial flag")
	}
	n := int(r.u16())
	if !r.bounded(n, 6) {
		return nil, fmt.Errorf("invalid known-spells count")
	}
	out := make([]uint32, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, r.u32())
		r.u16()
	}
	history := int(r.u16())
	if !r.bounded(history, 18) {
		return nil, fmt.Errorf("invalid spell history")
	}
	r.take(history * 18)
	return out, r.done()
}

// No combat is sent in this session. Quest discovery precedes the existing
// navigation controller; the next session independently verifies its endpoint.
func ApproachObjective434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, questID uint32, finder RouteFinder434, factions *NPCFactions434) (ObjectiveAttempt434, error) {
	a := ObjectiveAttempt434{Observation: QuestProgressObservation434{World: WorldState434Result{Opcodes: map[uint16]int{}, WorldVariables: map[uint32]int32{}}}}
	if finder == nil || factions == nil || questID == 0 {
		return a, fmt.Errorf("BAD_TEST: explicit objective/navigation inputs required")
	}
	ctx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()
	q := questProgressController434{result: &a.Observation, requested: questID}
	nav := navigationController434{ctx: ctx, result: &a.Navigation, finder: finder}
	nav.destination = func(s *WorldState434Result) (pathfinding.Point3D, error) {
		objective, err := singleObjective434(a.Observation.Quest, 0)
		if err != nil {
			return pathfinding.Point3D{}, err
		}
		a.Candidates = objectiveCandidates434(&s.Store, factions, objective.Target)
		if len(a.Candidates) == 0 {
			return pathfinding.Point3D{}, fmt.Errorf("TARGET_SELECTION_BUG: no eligible visible objective creature within 40 units")
		}
		p := s.Store.objects[s.Store.PlayerGUID]
		for _, candidate := range a.Candidates {
			dest, routeErr := npcDestination434(*p.Position, NPCCandidate434{Position: candidate.Position})
			if routeErr == nil {
				var route *pathfinding.PathResult
				route, routeErr = finder.FindPath(uint32(p.Map), point434(*p.Position), point434(dest))
				if routeErr == nil {
					_, routeErr = validateBoundedRoute434(point434(*p.Position), point434(dest), route, 40, 1, 50, 64)
				}
			}
			if routeErr == nil {
				a.Candidate = candidate
				return point434(dest), nil
			}
			a.RouteRejections = append(a.RouteRejections, fmt.Sprintf("%016X: %v", candidate.GUID, routeErr))
		}
		return pathfinding.Point3D{}, fmt.Errorf("NAVIGATION_ADAPTER_BUG: no candidate passes existing MMap safeguards: %v", a.RouteRejections)
	}
	nav.validate = func(s, d pathfinding.Point3D, r *pathfinding.PathResult) ([]Position434, error) {
		return validateBoundedRoute434(s, d, r, 40, 1, 50, 64)
	}
	observer := func(p loginPacket434) error {
		if p.op == 0x0104 {
			var err error
			a.KnownSpells, err = knownSpells434(p.body)
			if err != nil {
				return err
			}
		}
		if err := q.observe(p); err != nil {
			return err
		}
		a.Navigation.World = a.Observation.World
		return nil
	}
	tick := func(now uint32, send func(uint16, []byte) error) error {
		if !q.done {
			return q.tick(now, send)
		}
		live, err := ProjectQuestProgress434(a.Observation.Quest.Definition, &a.Observation.World.Store)
		if err != nil {
			return err
		}
		if _, err = singleObjective434(live, 0); err != nil {
			return err
		}
		p := a.Observation.World.Store.objects[a.Observation.World.Store.PlayerGUID]
		a.PlayerFields = p.Fields
		for _, index := range []uint16{0x8, 0x9, 0xa, 0xb, 0xe, 0xf, 0x10, 0x11} {
			if p.Fields[index] != 0 {
				return fmt.Errorf("BAD_TEST: controlled/summoned unit present (field 0x%X=%08X); pet combat requires source-confirmed control", index, p.Fields[index])
			}
		}
		u, err := combatUnit434(p)
		if err != nil {
			return err
		}
		a.Player = u
		if u.Dead || u.InCombat || uint64(u.Health)*100 < uint64(u.MaxHealth)*90 {
			return fmt.Errorf("BAD_TEST: approach requires healthy out-of-combat player")
		}
		a.Navigation.World = a.Observation.World
		return nav.tick(now, send)
	}
	err := objectiveSession434(ctx, user, key, realm, name, instance, &a.Observation.World, observer, func() (bool, error) { return nav.stage == 6, nil }, tick)
	a.Navigation.World = a.Observation.World
	return a, err
}

func objectiveSession434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, world *WorldState434Result, observe func(loginPacket434) error, done func() (bool, error), tick func(uint32, func(uint16, []byte) error) error) error {
	if name == "" || instance == "" {
		return fmt.Errorf("BAD_TEST: explicit character and instance required")
	}
	return withWorld434(ctx, user, key, realm, func(w *worldWire434) error {
		roster, err := enumerateOnWorld434(w)
		if err != nil {
			return err
		}
		character, err := selectLoginCharacter434(roster, name)
		if err != nil {
			return err
		}
		world.Login.Character = character
		world.Store.PlayerGUID = character.GUID
		if err = w.send(cataPlayerLogin, loginGUID434(character.GUID)); err != nil {
			return err
		}
		return awaitSession434(ctx, w, strings.ToUpper(user), instance, &world.Login, openInstance434, observe, done, tick)
	})
}

type objectiveCombatController434 struct {
	result              *ObjectiveAttempt434
	query               questProgressController434
	approach            *ObjectiveAttempt434
	factions            *NPCFactions434
	startedAt, finishAt uint32
	pending             error
}

func (c *objectiveCombatController434) observe(p loginPacket434) error {
	a := c.result
	// Store errors after attack must reach tick so attack-stop is still attempted.
	if err := c.query.observe(p); err != nil {
		c.pending = fmt.Errorf("PROTOCOL_HARNESS_BUG: %w", err)
		return nil
	}
	e, known, err := parseCombatEvent434(p.op, p.body)
	if err != nil {
		c.pending = err
		return nil
	}
	if known && (e.Attacker == a.Observation.World.Store.PlayerGUID || e.Victim == a.Observation.World.Store.PlayerGUID) {
		a.CombatEvents = append(a.CombatEvents, e)
		player := a.Observation.World.Store.PlayerGUID
		if e.Attacker == player && e.Victim != a.Candidate.GUID && !(e.Opcode == cataServerAttackStop434 && e.Victim == 0) {
			c.pending = fmt.Errorf("TARGET_SELECTION_BUG: unexpected player attack victim")
		}
		if e.Victim == player && e.Attacker != a.Candidate.GUID {
			c.pending = fmt.Errorf("COMBAT_STATE_BUG: additional attacker")
		}
		if e.Attacker == player {
			a.ObservedAttack = e.Opcode == cataAttackStart434
		}
	}
	if p.op == 0x6c07 || p.op == 0x0016 || p.op == 0x0b36 {
		c.pending = fmt.Errorf("COMBAT_STATE_BUG: server swing rejection 0x%04X", p.op)
	}
	if p.op == 0x2b26 && !a.CommandedAttack {
		c.pending = fmt.Errorf("COMBAT_STATE_BUG: dead target before attack")
	}
	return nil
}

func (c *objectiveCombatController434) send(send func(uint16, []byte) error, op uint16, b []byte) error {
	if err := send(op, b); err != nil {
		return err
	}
	c.result.Sent = append(c.result.Sent, op)
	return nil
}
func (c *objectiveCombatController434) stop(send func(uint16, []byte) error) error {
	a := c.result
	if a.Stopped || !a.CommandedAttack {
		return nil
	}
	if err := c.send(send, cataAttackStop434, nil); err != nil {
		return err
	}
	a.Stopped = true
	return c.send(send, cataSetSelection, targetGUID434(0))
}
func (c *objectiveCombatController434) tick(now uint32, send func(uint16, []byte) error) (err error) {
	a := c.result
	defer func() {
		if err != nil {
			if stopErr := c.stop(send); stopErr != nil {
				err = fmt.Errorf("%w; attack stop failed: %v", err, stopErr)
			}
		}
	}()
	if c.pending != nil {
		return c.pending
	}
	if !c.query.done {
		return c.query.tick(now, send)
	}
	q, err := ProjectQuestProgress434(a.Observation.Quest.Definition, &a.Observation.World.Store)
	if err != nil {
		return fmt.Errorf("QUEST_PROGRESS_BUG: %w", err)
	}
	a.Observation.Quest = q
	s := &a.Observation.World.Store
	p, n := s.objects[s.PlayerGUID], s.objects[a.Candidate.GUID]
	a.Player, err = combatUnit434(p)
	if err != nil {
		return err
	}
	if a.Player.Dead {
		return fmt.Errorf("COMBAT_STATE_BUG: Ghost died; STOP, no recovery")
	}
	if uint64(a.Player.Health)*100 <= uint64(a.Player.MaxHealth)*60 {
		return fmt.Errorf("COMBAT_STATE_BUG: 60 percent health danger threshold")
	}
	if n == nil || n.Fields[FieldEntry434] != a.Candidate.Entry || n.Map != p.Map {
		return fmt.Errorf("TARGET_SELECTION_BUG: selected target invalid")
	}
	a.Target, err = combatUnit434(n)
	if err != nil {
		return err
	}
	if len(a.HealthHistory) == 0 || a.HealthHistory[len(a.HealthHistory)-1] != a.Target {
		a.HealthHistory = append(a.HealthHistory, a.Target)
	}
	if a.Player.Target != 0 && a.Player.Target != a.Candidate.GUID {
		return fmt.Errorf("TARGET_SELECTION_BUG: player target changed")
	}
	if !a.CommandedAttack {
		if len(a.Observation.Events) != 0 {
			return fmt.Errorf("BAD_TEST: quest event before controlled attack")
		}
		for _, index := range []uint16{0x8, 0x9, 0xa, 0xb, 0xe, 0xf, 0x10, 0x11} {
			if p.Fields[index] != 0 {
				return fmt.Errorf("BAD_TEST: controlled/summoned unit present; pet combat is outside this milestone")
			}
		}
		if p.Fields[0x3f] != 0 || a.Player.Flags&(0x20000|0x40000|0x200000|0x400000|0x800000) != 0 {
			return fmt.Errorf("BAD_TEST: mounted or disabled player")
		}
		o, err := singleObjective434(q, 0)
		if err != nil {
			return err
		}
		if o.Target != a.Candidate.Entry {
			return fmt.Errorf("TARGET_SELECTION_BUG: definition target changed")
		}
		if _, _, err = ValidateNavigationProof434(c.approach.Navigation, a.Observation.World); err != nil {
			return fmt.Errorf("NAVIGATION_ADAPTER_BUG: %w", err)
		}
		if a.Target.Dead || a.Target.InCombat || a.Target.Target != 0 || a.Target.Health != a.Candidate.Health || a.Target.Dynamic&4 != 0 || a.Target.Flags&combatForbidden434 != 0 || a.Player.InCombat {
			return fmt.Errorf("BAD_TEST: target/player changed before attack")
		}
		if _, ok := c.factions.combatReaction434(n, p); !ok {
			return fmt.Errorf("TARGET_SELECTION_BUG: target no longer attackable")
		}
		if p.Position == nil || n.Position == nil || p.Movement == nil {
			return fmt.Errorf("COMBAT_STATE_BUG: missing positions")
		}
		r, err := meleeRange434(p, n)
		if err != nil {
			return err
		}
		if distance434(point434(*p.Position), point434(*n.Position)) > r-0.5 {
			return fmt.Errorf("COMBAT_STATE_BUG: target moved out of conservative melee range")
		}
		m := p.Movement.clone()
		m.Position.Orientation = float32(math.Atan2(float64(n.Position.Y-p.Position.Y), float64(n.Position.X-p.Position.X)))
		if m.Position.Orientation < 0 {
			m.Position.Orientation += 2 * math.Pi
		}
		m.Flags = 0
		m.Timestamp = now
		m.HasTimestamp = true
		m.HasOrientation = true
		b, err := encodeMovement434(cataMoveHeartbeat, m)
		if err != nil {
			return err
		}
		if err = c.send(send, cataSetActiveMover, activeMover434(p.GUID)); err != nil {
			return err
		}
		if err = c.send(send, cataMoveHeartbeat, b); err != nil {
			return err
		}
		if err = c.send(send, cataSetSelection, targetGUID434(n.GUID)); err != nil {
			return err
		}
		if err = c.send(send, cataAttackSwing434, targetGUID434(n.GUID)); err != nil {
			return err
		}
		a.CommandedAttack = true
		c.startedAt = now
		return nil
	}
	if len(q.Objectives) != 1 || !q.Objectives[0].Known || q.Objectives[0].Current > 1 {
		return fmt.Errorf("QUEST_PROGRESS_BUG: unexpected objective state or progress above one")
	}
	for _, e := range a.Observation.Events {
		if e.Opcode != cataQuestCredit || e.QuestID != q.Definition.ID || e.Target != a.Candidate.Entry || e.VictimGUID != a.Candidate.GUID || e.Count != 1 || e.Required != 6 {
			return fmt.Errorf("QUEST_PROGRESS_BUG: unexpected quest credit/event")
		}
		a.Credit = true
	}
	a.Death = a.Target.Health == 0 // dynamic dead alone may be feign death
	if a.Death || a.Credit || q.Objectives[0].Current == 1 {
		if err = c.stop(send); err != nil {
			return err
		}
		if c.finishAt == 0 {
			c.finishAt = now
		}
		if now-c.finishAt > 5000 {
			return fmt.Errorf("QUEST_PROGRESS_BUG: missing death/credit/counter/combat-stop evidence")
		}
		if a.Death && a.Credit && !a.ObservedAttack && !a.Player.InCombat && q.Objectives[0].Current == 1 && now-c.finishAt >= 1500 {
			if _, err = singleObjective434(q, 1); err != nil {
				return err
			}
			a.Passed = true
		}
		return nil
	}
	if !a.ObservedAttack && now-c.startedAt > 3000 {
		return fmt.Errorf("COMBAT_STATE_BUG: attack not accepted or stopped early")
	}
	if a.Target.Target != 0 && a.Target.Target != p.GUID {
		return fmt.Errorf("COMBAT_STATE_BUG: target engaged another unit")
	}
	if n.Position == nil || p.Position == nil {
		return fmt.Errorf("COMBAT_STATE_BUG: missing combat position")
	}
	rangeLimit, rangeErr := meleeRange434(p, n)
	if rangeErr != nil {
		return rangeErr
	}
	if distance434(point434(*p.Position), point434(*n.Position)) > rangeLimit {
		return fmt.Errorf("COMBAT_STATE_BUG: target left melee range; no chase")
	}
	if a.Target.Flags&combatForbidden434 != 0 {
		return fmt.Errorf("COMBAT_STATE_BUG: target became unattackable")
	}
	if now-c.startedAt > 45000 {
		return fmt.Errorf("COMBAT_STATE_BUG: bounded melee timeout")
	}
	return nil
}

func KillOneObjective434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, factions *NPCFactions434, approach ObjectiveAttempt434) (ObjectiveAttempt434, error) {
	a := ObjectiveAttempt434{Candidate: approach.Candidate, Observation: QuestProgressObservation434{World: WorldState434Result{Opcodes: map[uint16]int{}, WorldVariables: map[uint32]int32{}}}}
	if a.Candidate.GUID == 0 || factions == nil {
		return a, fmt.Errorf("BAD_TEST: missing approach")
	}
	userContext := ctx
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 65*time.Second)
	defer cancel()
	c := objectiveCombatController434{result: &a, approach: &approach, factions: factions}
	c.query = questProgressController434{result: &a.Observation, requested: approach.Observation.Quest.Definition.ID}
	tick := func(now uint32, send func(uint16, []byte) error) error {
		return c.tickContext(userContext, now, send)
	}
	err := objectiveSession434(ctx, user, key, realm, name, instance, &a.Observation.World, c.observe, func() (bool, error) { return a.Passed, nil }, tick)
	return a, err
}

func (c *objectiveCombatController434) tickContext(ctx context.Context, now uint32, send func(uint16, []byte) error) error {
	if err := ctx.Err(); err != nil {
		if stopErr := c.stop(send); stopErr != nil {
			return fmt.Errorf("%w; stop failed: %v", err, stopErr)
		}
		return err
	}
	return c.tick(now, send)
}
