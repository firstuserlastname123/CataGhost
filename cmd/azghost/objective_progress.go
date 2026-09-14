package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/azerothcore/AzerothGhost/client"
	"github.com/azerothcore/AzerothGhost/config"
	"github.com/azerothcore/AzerothGhost/pathfinding"
)

func runObjectiveProgress(c config.CLIConfig, realmName, world, name, instance string, id uint32) error {
	if c.Username == "" || c.Password == "" || c.DataDir == "" || realmName == "" || world == "" || name == "" || instance == "" {
		return fmt.Errorf("BAD_TEST: explicit QA credentials, endpoints, character and data required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	factions, err := client.LoadNPCFactions434(filepath.Join(c.DataDir, "dbc", "enUS"))
	if err != nil {
		return err
	}
	login := func() (*client.AuthClient, client.RealmInfo, error) {
		a := client.NewAuthClient(c.Username, c.Password)
		realms, err := a.Authenticate(c.AuthServer)
		if err != nil {
			return nil, client.RealmInfo{}, fmt.Errorf("BAD_TEST: auth unavailable: %w", err)
		}
		r, err := selectWorldAuthRealm(realms, realmName, world)
		return a, r, err
	}
	dump := func(label string, v any) {
		b, e := json.Marshal(v)
		if e == nil {
			fmt.Printf("%s=%s\n", label, b)
		}
	}
	auth, realm, err := login()
	if err != nil {
		return err
	}
	approach, err := client.ApproachObjective434(ctx, c.Username, auth.SessionKey(), realm, name, instance, id, pathfinding.NewCataclysm434Navigator(filepath.Join(c.DataDir, "mmaps")), factions)
	dump("Approach", approach)
	if err != nil {
		return err
	}
	fmt.Println("Approach closed. Waiting for cleanup before independent endpoint and zero-progress verification.")
	if err = waitQuestCleanup(ctx); err != nil {
		return err
	}
	auth, realm, err = login()
	if err != nil {
		return err
	}
	attempt, err := client.KillOneObjective434(ctx, c.Username, auth.SessionKey(), realm, name, instance, factions, approach)
	dump("Combat", attempt)
	if err != nil {
		return err
	}
	if !attempt.Passed {
		return fmt.Errorf("COMBAT_STATE_BUG: missing one-credit proof")
	}
	fmt.Println("Exactly one death and 1/6 observed; combat stopped. Waiting for cleanup before read-only persistence verification.")
	if err = waitQuestCleanup(ctx); err != nil {
		return err
	}
	auth, realm, err = login()
	if err != nil {
		return err
	}
	after, err := client.ObserveQuestProgress434(ctx, c.Username, auth.SessionKey(), realm, name, instance, id)
	if err != nil {
		return err
	}
	printQuestProgress434("Persistence", after)
	if err = client.VerifyQuestProgressReconnect434(attempt.Observation, after); err != nil {
		return fmt.Errorf("QUEST_PROGRESS_BUG: %w", err)
	}
	fmt.Println("PASS: exactly one selected creature death, authoritative 0/6 -> 1/6, persistent across reconnect, Ghost survived, clean disconnect. No second target, loot or turn-in.")
	return nil
}
