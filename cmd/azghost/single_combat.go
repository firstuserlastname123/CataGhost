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

// runSingleCombat434 performs one explicitly requested controlled attack. It
// never selects a second target and contains no loot or spell send path.
func runSingleCombat434(c config.CLIConfig, realmName, world, name, instance string) error {
	if c.Username == "" || c.Password == "" || c.DataDir == "" || realmName == "" || world == "" || name == "" || instance == "" {
		return fmt.Errorf("BAD_TEST: explicit credentials, endpoints, character and data required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	a := client.NewAuthClient(c.Username, c.Password)
	realms, err := a.Authenticate(c.AuthServer)
	if err != nil {
		return err
	}
	realm, err := selectWorldAuthRealm(realms, realmName, world)
	if err != nil {
		return err
	}
	factions, err := client.LoadNPCFactions434(filepath.Join(c.DataDir, "dbc", "enUS"))
	if err != nil {
		return err
	}
	finder := pathfinding.NewCataclysm434Navigator(filepath.Join(c.DataDir, "mmaps"))
	snapshot, err := client.ObserveWorld434(ctx, c.Username, a.SessionKey(), realm, name, instance)
	if err != nil {
		return err
	}
	choices, err := client.InspectSingleCombatTargets434(snapshot, finder, factions)
	if err != nil {
		return err
	}
	if len(choices) == 0 || !choices[0].Eligible {
		return fmt.Errorf("BAD_TEST: no route-valid isolated attackable creature; no combat sent")
	}
	target := choices[0].Candidate
	fmt.Printf("SingleCombatTarget GUID=%016X Entry=%d Distance=%.2f Clearance=%.2f Route=%.2f Reaction=%q\n", target.GUID, target.Entry, target.Distance, choices[0].MinimumClearance, choices[0].RouteLength, target.Reaction)
	move, err := client.NavigateCharacterCombatApproach434(ctx, c.Username, a.SessionKey(), realm, name, instance, finder, target.GUID)
	if err != nil {
		return err
	}
	fmt.Printf("CombatApproach segments=%d sent=%v\n", move.Executed, move.Sent)
	result, err := client.ExecuteSingleCombat434(ctx, c.Username, a.SessionKey(), realm, name, instance, factions, move, target.GUID)
	if err != nil {
		return err
	}
	b, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return marshalErr
	}
	fmt.Printf("SingleCombat=%s\n", b)
	fmt.Println("PASS: exactly one server-controlled creature reached authoritative health 0; player alive; no loot attempted; clean shutdown.")
	return nil
}
