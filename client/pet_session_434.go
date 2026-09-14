package client

import (
	"context"
	"fmt"
	"time"
)

type PetSafetyObservation434 struct {
	Player                    CombatUnit434
	ControlGate               PetControlGate434
	PlayerAuras               []AuraPacket434
	Observation               QuestProgressObservation434
	Pet                       PetState434
	InitialPet                *PetState434
	Modes                     []PetMode434
	Sent                      []uint16
	ControlSent, SafetyProven bool
	PetStatePackets           int
}

type petSafetyController434 struct {
	auras                       auraObserver434
	readOnly                    bool
	result                      *PetSafetyObservation434
	query                       questProgressController434
	initialization              *PetSpells434
	requestAt                   uint32
	requestedVersion            int
	requested, controlled, done bool
}

func (c *petSafetyController434) observe(p loginPacket434) error {
	if err := c.query.observe(p); err != nil {
		return err
	}
	switch p.op {
	case cataAuraUpdate434, cataAuraUpdateAll434:
		update, err := parseAuraPacket434(p.op, p.body)
		if err != nil {
			return err
		}
		if update.GUID == c.result.Observation.World.Store.PlayerGUID {
			c.auras.apply(update)
			c.result.PlayerAuras = append(c.result.PlayerAuras, update)
		}
	case cataPetSpells434:
		state, err := parsePetSpells434(p.body)
		if err != nil {
			return err
		}
		c.initialization = &state
		c.result.PetStatePackets++
	case cataPetMode434:
		mode, err := parsePetMode434(p.body)
		if err != nil {
			return fmt.Errorf("PET_STATE_DECODER_BUG: %w", err)
		}
		c.result.Modes = append(c.result.Modes, mode)
	}
	return nil
}

func (c *petSafetyController434) tick(now uint32, send func(uint16, []byte) error) error {
	if c.done {
		return nil
	}
	if !c.query.done {
		return c.query.tick(now, send)
	}
	a := c.result
	q, err := ProjectQuestProgress434(a.Observation.Quest.Definition, &a.Observation.World.Store)
	if err != nil {
		return err
	}
	a.Observation.Quest = q
	if _, err = singleObjective434(q, 0); err != nil {
		return err
	}
	if len(a.Observation.Events) != 0 {
		return fmt.Errorf("BAD_TEST: quest event during stationary pet probe")
	}
	player, err := combatUnit434(a.Observation.World.Store.objects[a.Observation.World.Store.PlayerGUID])
	if err != nil {
		return err
	}
	a.Player = player
	if player.Dead || player.InCombat || uint64(player.Health)*100 < uint64(player.MaxHealth)*90 {
		return fmt.Errorf("BAD_TEST: pet probe requires healthy non-combat player")
	}
	a.Pet, err = projectPetState434(&a.Observation.World.Store, c.initialization, c.controlled && a.PetStatePackets > c.requestedVersion)
	// 93321 is a passive dummy aura in local 15595 DBC data. The TCPP
	// CanBeSentToClient predicate excludes it; missing visible slots are UNKNOWN.
	presence := c.auras.presence(a.Observation.World.Store.PlayerGUID, hunterControlAura434, false)
	a.ControlGate = petControlGate434(a.Observation.World.Login.Character.Class, a.Pet, presence)
	if err != nil {
		return err
	}
	if !a.Pet.Active {
		a.SafetyProven = a.Pet.Safe
		c.done = true
		return nil
	}
	if a.Pet.Unit.Dead || a.Pet.Unit.InCombat || a.Pet.Unit.Target != 0 {
		return fmt.Errorf("UNKNOWN: active pet is dead, in combat or targeted")
	}
	transmit := func(op uint16, b []byte) error {
		if err := send(op, b); err != nil {
			return err
		}
		a.Sent = append(a.Sent, op)
		return nil
	}
	if !c.requested {
		if err := transmit(cataRequestPetInfo434, nil); err != nil {
			return err
		}
		c.requested = true
		c.requestAt = now
		c.requestedVersion = a.PetStatePackets
		return nil
	}
	if a.PetStatePackets <= c.requestedVersion {
		if now-c.requestAt >= 5000 {
			return fmt.Errorf("UNKNOWN: identified pet %016X, but no authoritative pet-info response to 0x4924; reaction and command not verified", a.Pet.GUID)
		}
		return nil
	}
	if !a.Pet.StateKnown {
		return fmt.Errorf("UNKNOWN: pet info cleared without authoritative reaction/command")
	}
	if c.readOnly {
		c.done = true
		return nil
	}
	if a.InitialPet == nil {
		snapshot := a.Pet
		a.InitialPet = &snapshot
	}
	if a.Pet.Safe {
		a.SafetyProven = true
		c.done = true
		return nil
	}
	if c.controlled {
		return fmt.Errorf("PET_CONTROL_BUG: refreshed pet state is not PASSIVE/FOLLOW/non-attacking")
	}
	// A positive response demonstrates a supported authoritative readback path.
	// With no response, never send a command whose result cannot be verified.
	actions := []bool{true}
	if a.Pet.Initialization.Reaction != 0 {
		actions = []bool{false, true}
	}
	for _, follow := range actions {
		body, err := safePetAction434(a.Pet.GUID, follow)
		if err != nil {
			return err
		}
		if err = transmit(cataPetAction434, body); err != nil {
			return err
		}
		a.ControlSent = true
	}
	c.controlled = true
	c.initialization = nil // no commanded state may masquerade as observed state
	a.Pet.StateKnown = false
	a.Pet.Reaction = "UNKNOWN"
	a.Pet.Command = "UNKNOWN"
	a.Pet.Safe = false
	if err = transmit(cataRequestPetInfo434, nil); err != nil {
		return err
	}
	c.requestAt = now
	c.requestedVersion = a.PetStatePackets
	return nil
}

// ObservePetSafety434 is stationary. The only possible gameplay mutation is
// PASSIVE + FOLLOW on an identified pet with a proven pet-info readback path.
// No player/pet attack, movement, spell cast, dismissal or loot is reachable.
func ObservePetSafety434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, questID uint32) (PetSafetyObservation434, error) {
	return observePetState434(ctx, user, key, realm, name, instance, questID, false)
}

// ObservePetControlGate434 has no reachable control/movement/combat branch.
func ObservePetControlGate434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, questID uint32) (PetSafetyObservation434, error) {
	return observePetState434(ctx, user, key, realm, name, instance, questID, true)
}

func observePetState434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, questID uint32, readOnly bool) (PetSafetyObservation434, error) {
	a := PetSafetyObservation434{Observation: QuestProgressObservation434{World: WorldState434Result{Opcodes: map[uint16]int{}, WorldVariables: map[uint32]int32{}}}}
	if questID == 0 {
		return a, fmt.Errorf("BAD_TEST: explicit active quest required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c := petSafetyController434{result: &a, readOnly: readOnly}
	c.query = questProgressController434{result: &a.Observation, requested: questID}
	err := objectiveSession434(ctx, user, key, realm, name, instance, &a.Observation.World, c.observe, func() (bool, error) { return c.done, nil }, c.tick)
	return a, err
}
