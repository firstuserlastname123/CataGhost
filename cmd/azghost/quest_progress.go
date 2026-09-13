package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/azerothcore/AzerothGhost/client"
	"github.com/azerothcore/AzerothGhost/config"
)

func runQuestProgress(c config.CLIConfig, realmName, world, name, instance string, questID uint32) error {
	if c.Username == "" || c.Password == "" || realmName == "" || world == "" || name == "" || instance == "" {
		return fmt.Errorf("explicit credentials, realm, world, character and instance required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	observe := func(id uint32) (client.QuestProgressObservation434, error) {
		a := client.NewAuthClient(c.Username, c.Password)
		realms, err := a.Authenticate(c.AuthServer)
		if err != nil {
			return client.QuestProgressObservation434{}, err
		}
		realm, err := selectWorldAuthRealm(realms, realmName, world)
		if err != nil {
			return client.QuestProgressObservation434{}, err
		}
		return client.ObserveQuestProgress434(ctx, c.Username, a.SessionKey(), realm, name, instance, id)
	}
	fmt.Println("=== CataGhost Quest Objective Tracking (4.3.4 / 15595) ===")
	first, err := observe(questID)
	if err != nil {
		return err
	}
	printQuestProgress434("Initial", first)
	fmt.Println("Stationary observation closed. Waiting for TCPP cleanup before independent reconnect.")
	if err := waitQuestCleanup(ctx); err != nil {
		return err
	}
	second, err := observe(first.Quest.Definition.ID)
	if err != nil {
		return err
	}
	printQuestProgress434("Reconnect", second)
	if err := client.VerifyQuestProgressReconnect434(first, second); err != nil {
		return err
	}
	fmt.Println("PASS: live requirements and authoritative quest state agree across reconnect. No movement, combat, loot, accept or turn-in sent. Nonzero progress deferred to a separately verified objective-action subsystem. Clean shutdown.")
	return nil
}

func printQuestProgress434(label string, o client.QuestProgressObservation434) {
	q := o.Quest
	fmt.Printf("%s player=%016X map=%d quest=%d title=%q slot=%d status=%s state=0x%X counters=%v timer=%d timeSync=%d\n", label, o.World.Store.PlayerGUID, o.World.Login.Map, q.Definition.ID, q.Definition.Title, q.Slot.Slot, q.Status, q.Slot.State, q.Slot.Counters, q.Slot.Timer, o.World.Login.TimeSyncCounter)
	fmt.Printf("Summary=%q type=%d level=%d minimumLevel=%d flags=0x%X\n", q.Definition.Summary, q.Definition.Type, q.Definition.Level, q.Definition.MinLevel, q.Definition.Flags)
	for _, p := range q.Objectives {
		fmt.Printf("Objective index=%d type=%s target=%d required=%d current=%d known=%t complete=%t source=%q text=%q\n", p.Index, p.Kind, p.Target, p.Required, p.Current, p.Known, p.Complete, p.Source, p.Text)
	}
	fmt.Printf("New client opcodes=%04X; progress events=%+v; observed opcodes=%v\n", o.Sent, o.Events, o.World.Opcodes)
	b, _ := json.Marshal(q)
	fmt.Printf("Quest snapshot JSON=%s\n", b)
}
