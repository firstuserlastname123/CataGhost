package client

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const playerDangerPercent434 = 30

type SingleCombatResult434 struct {
	World                  WorldState434Result
	Target                 ObjectiveCandidate434
	Player, TargetState    CombatUnit434
	Sent                   []uint16
	AttackStarted          bool
	AuthoritativeDeath     bool
	ServerCombatTerminated bool
	SelectedTargets        int
}

func selectionPacket434(guid uint64) []byte   { return targetGUID434(guid) }
func attackSwingPacket434(guid uint64) []byte { return targetGUID434(guid) }
func attackStopPacket434() []byte             { return nil }

type singleCombatController434 struct {
	result                  *SingleCombatResult434
	target                  uint64
	readyAt, sentAt, deadAt uint32
	stopSent                bool
}

func (c *singleCombatController434) stop(send func(uint16, []byte) error) error {
	if c.stopSent {
		return nil
	}
	c.stopSent = true
	if err := send(cataAttackStop434, attackStopPacket434()); err != nil {
		return err
	}
	c.result.Sent = append(c.result.Sent, cataAttackStop434)
	return nil
}

func (c *singleCombatController434) states() (CombatUnit434, CombatUnit434, error) {
	s := &c.result.World.Store
	p, n := s.objects[s.PlayerGUID], s.objects[c.target]
	pu, err := combatUnit434(p)
	if err != nil {
		return pu, CombatUnit434{}, err
	}
	tu, err := combatUnit434(n)
	if err != nil {
		return pu, tu, err
	}
	return pu, tu, nil
}

func (c *singleCombatController434) safety(send func(uint16, []byte) error) error {
	pu, tu, err := c.states()
	if err != nil {
		return err
	}
	c.result.Player, c.result.TargetState = pu, tu
	if pu.Dead || uint64(pu.Health)*100 <= uint64(pu.MaxHealth)*playerDangerPercent434 {
		_ = c.stop(send)
		return fmt.Errorf("BAD_TEST: player danger abort at %d/%d", pu.Health, pu.MaxHealth)
	}
	for _, o := range c.result.World.Store.Objects() {
		if o.GUID == pu.GUID || o.GUID == c.target || o.Type != 3 {
			continue
		}
		u, e := combatUnit434(&o)
		if e == nil && !u.Dead && u.InCombat && u.Target == pu.GUID {
			_ = c.stop(send)
			return fmt.Errorf("BAD_TEST: second creature %016X joined controlled fight", o.GUID)
		}
	}
	return nil
}

func (c *singleCombatController434) observe(p loginPacket434) error {
	if err := c.result.World.observe(p); err != nil {
		return err
	}
	e, known, err := parseCombatEvent434(p.op, p.body)
	if err != nil {
		return err
	}
	if !known {
		return nil
	}
	player := c.result.World.Store.PlayerGUID
	if e.Opcode == cataAttackStart434 {
		if e.Attacker == player && e.Victim == c.target {
			c.result.AttackStarted = true
		} else if e.Attacker == player || e.Victim == player {
			return fmt.Errorf("COMBAT_STATE_BUG: unexpected combat participant %016X/%016X", e.Attacker, e.Victim)
		}
	}
	if e.Opcode == cataServerAttackStop434 && ((e.Attacker == player && e.Victim == c.target) || (e.Attacker == c.target && e.Victim == player)) {
		c.result.ServerCombatTerminated = true
	}
	return nil
}

func (c *singleCombatController434) tick(now uint32, send func(uint16, []byte) error) error {
	if c.sentAt != 0 {
		if err := c.safety(send); err != nil {
			return err
		}
		_, target, _ := c.states()
		if target.Health == 0 {
			c.result.AuthoritativeDeath = true
			if c.deadAt == 0 {
				c.deadAt = now
				if err := c.stop(send); err != nil {
					return err
				}
			}
		}
		if !c.result.AuthoritativeDeath && now-c.sentAt > 45000 {
			_ = c.stop(send)
			return fmt.Errorf("BAD_TEST: controlled combat timed out")
		}
		return nil
	}
	ready, err := c.result.World.playerReady()
	if err != nil || !ready {
		return err
	}
	if c.readyAt == 0 {
		c.readyAt = now
		return nil
	}
	if now-c.readyAt < 1000 {
		return nil
	}
	pu, tu, err := c.states()
	if err != nil {
		return err
	}
	if pu.Dead || pu.Health != pu.MaxHealth || pu.InCombat || pu.Target != 0 || tu.Dead || tu.Health != tu.MaxHealth || tu.InCombat || tu.Target != 0 {
		return fmt.Errorf("BAD_TEST: ambiguous precombat state")
	}
	if err := send(cataSetSelection, selectionPacket434(c.target)); err != nil {
		return err
	}
	c.result.Sent = append(c.result.Sent, cataSetSelection)
	c.result.SelectedTargets = 1
	if err := send(cataAttackSwing434, attackSwingPacket434(c.target)); err != nil {
		return err
	}
	c.result.Sent = append(c.result.Sent, cataAttackSwing434)
	c.sentAt = now
	return nil
}

func (c *singleCombatController434) complete() (bool, error) {
	if c.result.AuthoritativeDeath && c.result.AttackStarted && c.result.ServerCombatTerminated {
		p, _, err := c.states()
		if err != nil {
			return false, err
		}
		if p.Dead {
			return false, fmt.Errorf("COMBAT_STATE_BUG: player died")
		}
		return true, nil
	}
	return false, nil
}

func validateCombatArrival434(world WorldState434Result, factions *NPCFactions434, target uint64) error {
	if err := VerifyUncontrolledPlayer434(&world.Store); err != nil {
		return err
	}
	p, n := world.Store.objects[world.Store.PlayerGUID], world.Store.objects[target]
	if p == nil || p.Position == nil || n == nil || n.Position == nil || n.Type != 3 || n.GUID>>52 != 0xf13 || n.Map != p.Map || !finitePosition434(n.Position) || n.Fields[FieldNPCFlags434] != 0 {
		return fmt.Errorf("BAD_TEST: selected creature unavailable after approach")
	}
	u, err := combatUnit434(n)
	if err != nil || u.Dead || u.InCombat || u.Target != 0 || u.Health != u.MaxHealth || u.Flags&combatForbidden434 != 0 || u.Dynamic&4 != 0 {
		return fmt.Errorf("BAD_TEST: selected creature state changed after approach")
	}
	if _, ok := factions.combatReaction434(n, p); !ok {
		return fmt.Errorf("BAD_TEST: selected creature is not attackable")
	}
	rangeLimit, err := meleeRange434(p, n)
	if err != nil {
		return err
	}
	if distance434(point434(*p.Position), point434(*n.Position)) > rangeLimit {
		return fmt.Errorf("BAD_TEST: selected creature is outside melee range")
	}
	for _, other := range world.Store.Objects() {
		if other.GUID == p.GUID || other.GUID == target || other.Type != 3 || other.Map != p.Map {
			continue
		}
		if other.Position == nil || !finitePosition434(other.Position) {
			return fmt.Errorf("UNKNOWN: nearby creature position unavailable")
		}
		ou, e := combatUnit434(&other)
		if e == nil && ou.Dead {
			continue
		}
		if e == nil && ou.InCombat && ou.Target == p.GUID {
			return fmt.Errorf("BAD_TEST: player already has another combat participant")
		}
		reaction, attackable := factions.combatReaction434(&other, p)
		if !attackable && reaction == "friendly" {
			continue
		}
		if distance434(point434(*other.Position), point434(*p.Position)) <= objectiveIsolationRadius434 || distance434(point434(*other.Position), point434(*n.Position)) <= objectiveIsolationRadius434 {
			return fmt.Errorf("BAD_TEST: isolation changed after approach")
		}
	}
	return nil
}

// ExecuteSingleCombat434 reconnects after a proven guarded approach, requires
// the selected GUID to remain isolated and in melee range, then sends exactly
// one selection and one swing.
func ExecuteSingleCombat434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, factions *NPCFactions434, approach NavigationAttempt434, target uint64) (SingleCombatResult434, error) {
	r := SingleCombatResult434{World: WorldState434Result{Opcodes: map[uint16]int{}, WorldVariables: map[uint32]int32{}}}
	if name == "" || instance == "" || factions == nil || target == 0 || approach.Executed < 1 || approach.Executed != len(approach.Segments) {
		return r, fmt.Errorf("BAD_TEST: explicit combat inputs required")
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
		r.World.Login.Character = character
		r.World.Store.PlayerGUID = character.GUID
		if err := w.send(cataPlayerLogin, loginGUID434(character.GUID)); err != nil {
			return err
		}
		ctl := singleCombatController434{result: &r, target: target}
		checked := false
		tick := func(now uint32, send func(uint16, []byte) error) error {
			if !checked {
				ready, e := r.World.playerReady()
				if e != nil || !ready {
					return e
				}
				if approach.World.Store.PlayerGUID != r.World.Store.PlayerGUID || approach.World.Login.Character.Name != name {
					return fmt.Errorf("BAD_TEST: combat approach identity mismatch")
				}
				if e := validateCombatArrival434(r.World, factions, target); e != nil {
					return e
				}
				n := r.World.Store.objects[target]
				u, _ := combatUnit434(n)
				r.Target = ObjectiveCandidate434{GUID: n.GUID, Entry: n.Fields[FieldEntry434], Faction: n.Fields[FieldFaction434], Health: u.Health, MaxHealth: u.MaxHealth, Position: *n.Position, Distance: distance434(point434(*n.Position), point434(*r.World.Store.objects[r.World.Store.PlayerGUID].Position))}
				checked = true
			}
			return ctl.tick(now, send)
		}
		return awaitSession434(ctx, w, strings.ToUpper(user), instance, &r.World.Login, openInstance434, ctl.observe, ctl.complete, tick)
	})
	return r, err
}
