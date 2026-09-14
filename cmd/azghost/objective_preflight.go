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

// This stationary command has no movement or combat send path.
func runObjectivePreflight434(c config.CLIConfig, realmName, world, name, instance string, id uint32) error {
	if c.Username == "" || c.Password == "" || c.DataDir == "" || realmName == "" || world == "" || name == "" || instance == "" || id == 0 {
		return fmt.Errorf("BAD_TEST: explicit credentials, endpoints, character, quest and data required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	a := client.NewAuthClient(c.Username, c.Password)
	realms, err := a.Authenticate(c.AuthServer)
	if err != nil {
		return err
	}
	r, err := selectWorldAuthRealm(realms, realmName, world)
	if err != nil {
		return err
	}
	o, err := client.ObserveQuestProgress434(ctx, c.Username, a.SessionKey(), r, name, instance, id)
	if err != nil {
		return err
	}
	printWorldState434(os.Stdout, o.World)
	printQuestProgress434("Preflight", o)
	if err := client.VerifyUncontrolledPlayer434(&o.World.Store); err != nil {
		return err
	}
	factions, err := client.LoadNPCFactions434(filepath.Join(c.DataDir, "dbc", "enUS"))
	if err != nil {
		return err
	}
	checks, err := client.InspectObjectiveIsolation434(o, pathfinding.NewCataclysm434Navigator(filepath.Join(c.DataDir, "mmaps")), factions)
	if err != nil {
		return err
	}
	b, err := json.Marshal(struct {
		Observation client.QuestProgressObservation434
		Objects     []client.Object434
		Isolation   []client.ObjectiveIsolation434
	}{o, o.World.Store.Objects(), checks})
	if err != nil {
		return err
	}
	fmt.Printf("ObjectivePreflight=%s\n", b)
	for _, check := range checks {
		if check.Eligible {
			fmt.Println("Route/isolation snapshot eligible; no combat milestone PASS claimed. Clean shutdown.")
			return nil
		}
	}
	return fmt.Errorf("BAD_TEST: no route-valid isolated objective candidate; no combat sent; clean shutdown")
}
