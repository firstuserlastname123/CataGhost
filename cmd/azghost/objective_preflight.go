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

// This command performs bounded observation movement. It has no combat send
// path: only the human-operated follow-up may authorize a one-kill attempt.
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
	factions, err := client.LoadNPCFactions434(filepath.Join(c.DataDir, "dbc", "enUS"))
	if err != nil {
		return err
	}
	finder := pathfinding.NewCataclysm434Navigator(filepath.Join(c.DataDir, "mmaps"))
	result, err := client.ScoutObjective434(ctx, finder, factions,
		func(ctx context.Context) (client.QuestProgressObservation434, error) {
			o, observeErr := client.ObserveQuestProgress434(ctx, c.Username, a.SessionKey(), r, name, instance, id)
			if observeErr == nil {
				printWorldState434(os.Stdout, o.World)
				printQuestProgress434("Scout", o)
			}
			return o, observeErr
		},
		func(ctx context.Context, position client.Position434) error {
			destination := pathfinding.Point3D{X: position.X, Y: position.Y, Z: position.Z}
			attempt, moveErr := client.NavigateCharacterTo434(ctx, c.Username, a.SessionKey(), r, name, instance, finder, &destination)
			if moveErr != nil {
				return moveErr
			}
			fmt.Printf("ScoutMove destination=%+v segments=%d sent=%v\n", position, attempt.Executed, attempt.Sent)
			return nil
		})
	if err != nil {
		return err
	}
	b, err := json.Marshal(struct {
		Observation client.QuestProgressObservation434
		Objects     []client.Object434
		Scout       client.ObjectiveScoutResult434
	}{result.Observation, result.Observation.World.Store.Objects(), result})
	if err != nil {
		return err
	}
	fmt.Printf("ObjectivePreflight=%s\n", b)
	fmt.Println("Route/isolation snapshot eligible after bounded scouting; no combat milestone PASS claimed. Clean shutdown.")
	return nil
}
