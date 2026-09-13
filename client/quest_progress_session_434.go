package client

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type QuestProgressObservation434 struct {
	World  WorldState434Result
	Quest  QuestProgress434
	Events []QuestProgressEvent434
	Sent   []uint16
}

type questProgressController434 struct {
	result           *QuestProgressObservation434
	requested        uint32
	selected         uint32
	definition       *QuestDefinition434
	readyAt, queryAt uint32
	done             bool
}

func (c *questProgressController434) observe(p loginPacket434) error {
	if err := c.result.World.observe(p); err != nil {
		return err
	}
	if p.op == cataQuestQueryResponse {
		d, err := parseQuestDefinition434(p.body)
		if err != nil {
			return err
		}
		if c.selected == 0 || d.ID != c.selected || c.definition != nil {
			return fmt.Errorf("unexpected quest definition identity/duplicate")
		}
		c.definition = &d
	}
	e, known, err := parseQuestProgressEvent434(p.op, p.body)
	if err != nil {
		return err
	}
	if known {
		if len(c.result.Events) >= 128 {
			return fmt.Errorf("quest event limit exceeded")
		}
		c.result.Events = append(c.result.Events, e)
	}
	return nil
}

func (c *questProgressController434) tick(now uint32, send func(uint16, []byte) error) error {
	if c.done {
		return nil
	}
	if c.selected == 0 {
		ready, err := c.result.World.playerReady()
		if err != nil {
			return err
		}
		if !ready {
			return nil
		}
		if c.readyAt == 0 {
			c.readyAt = now
			return nil
		}
		if now-c.readyAt < 1000 {
			return nil
		}
		slots, err := QuestLog434(&c.result.World.Store)
		if err != nil {
			return err
		}
		if c.requested == 0 && len(slots) != 1 {
			return fmt.Errorf("need exactly one active quest or an explicit quest ID; observed %d", len(slots))
		}
		for _, q := range slots {
			if c.requested == 0 || q.ID == c.requested {
				c.selected = q.ID
				break
			}
		}
		if c.selected == 0 {
			return fmt.Errorf("requested quest not in authoritative player log")
		}
		if err := send(cataQuestQueryInfo, questInfoRequest434(c.selected)); err != nil {
			return err
		}
		c.result.Sent = append(c.result.Sent, cataQuestQueryInfo)
		c.queryAt = now
		return nil
	}
	if now-c.queryAt > 8000 {
		return fmt.Errorf("quest definition timeout")
	}
	if c.definition != nil && now-c.queryAt >= 1500 {
		q, err := ProjectQuestProgress434(*c.definition, &c.result.World.Store)
		if err != nil {
			return err
		}
		c.result.Quest = q
		c.done = true
	}
	return nil
}

// ObserveQuestProgress434 is stationary and read-only. The one new outgoing
// packet queries immutable requirements, never giver details or acceptance.
func ObserveQuestProgress434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, questID uint32) (QuestProgressObservation434, error) {
	out := QuestProgressObservation434{World: WorldState434Result{Opcodes: map[uint16]int{}, WorldVariables: map[uint32]int32{}}}
	if name == "" || instance == "" {
		return out, fmt.Errorf("explicit character and instance required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c := questProgressController434{result: &out, requested: questID}
	err := withWorld434(ctx, user, key, realm, func(w *worldWire434) error {
		roster, err := enumerateOnWorld434(w)
		if err != nil {
			return err
		}
		character, err := selectLoginCharacter434(roster, name)
		if err != nil {
			return err
		}
		out.World.Login.Character = character
		out.World.Store.PlayerGUID = character.GUID
		if err := w.send(cataPlayerLogin, loginGUID434(character.GUID)); err != nil {
			return err
		}
		return awaitSession434(ctx, w, strings.ToUpper(user), instance, &out.World.Login, openInstance434, c.observe, func() (bool, error) { return c.done, nil }, c.tick)
	})
	return out, err
}

// The observer has issued no objective action, so the QA probe expects a stable
// snapshot. A real delta is reported for investigation, never overwritten.
func VerifyQuestProgressReconnect434(before, after QuestProgressObservation434) error {
	if before.World.Store.PlayerGUID == 0 || before.World.Store.PlayerGUID != after.World.Store.PlayerGUID || before.World.Login.Character.GUID != after.World.Login.Character.GUID || before.World.Login.Map != after.World.Login.Map {
		return fmt.Errorf("quest progress reconnect identity/map mismatch")
	}
	if !reflect.DeepEqual(before.Quest, after.Quest) {
		return fmt.Errorf("authoritative quest definition/progress changed across reconnect")
	}
	return nil
}
