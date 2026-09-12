package main

import (
	"context"
	"fmt"
	"github.com/azerothcore/AzerothGhost/client"
	"github.com/azerothcore/AzerothGhost/config"
	"github.com/azerothcore/AzerothGhost/pathfinding"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

func runNPCInteraction(c config.CLIConfig, realmName, worldAddress, name, instance string) error {
	if c.Username == "" || c.Password == "" || c.DataDir == "" || realmName == "" || worldAddress == "" || name == "" || instance == "" {
		return fmt.Errorf("NPC probe requires explicit credentials/realm/world/character/instance/data-dir")
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
			return nil, client.RealmInfo{}, err
		}
		r, err := selectWorldAuthRealm(realms, realmName, worldAddress)
		return a, r, err
	}
	fmt.Println("=== CataGhost NPC Interaction (4.3.4 / 15595) ===")
	auth, realm, err := login()
	if err != nil {
		return err
	}
	attempt, err := client.ApproachNPC434(ctx, c.Username, auth.SessionKey(), realm, name, instance, pathfinding.NewCataclysm434Navigator(filepath.Join(c.DataDir, "mmaps")), factions)
	if err != nil {
		fmt.Printf("Approach incomplete: segments=%d opcodes=%04X\n", attempt.Navigation.Executed, attempt.Navigation.Sent)
		return err
	}
	n, a := attempt.NPC, attempt.Navigation
	fmt.Printf("Realm=%q ID=%d Character=%q GUID=%016X map=%d\n", realm.Name, realm.ID, a.World.Login.Character.Name, a.Initial.GUID, a.World.Login.Map)
	fmt.Printf("NPC GUID=%016X entry=%d flags=0x%X faction=%d health=%d XYZ=(%.6f, %.6f, %.6f) initial distance=%.6f\n", n.GUID, n.Entry, n.Flags, n.Faction, n.Health, n.Position.X, n.Position.Y, n.Position.Z, n.Distance)
	fmt.Printf("Initial=%+v destination=%+v stop=%+v\n", a.Initial.Position, a.Requested, a.Commanded)
	fmt.Printf("MMap type=0x%X points=%d length=%.6f segments=%d opcodes=%04X\n", a.RawPath.Type, len(a.RawPath.Points), a.RawPath.PathLength(), a.Executed, a.Sent)
	for i, p := range a.RawPath.Points {
		fmt.Printf("path[%d]=%+v\n", i, p)
	}
	fmt.Println("Approach closed cleanly; waiting 65 seconds for server cleanup, then fresh position verification before interaction.")
	timer := time.NewTimer(65 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	auth, realm, err = login()
	if err != nil {
		return err
	}
	result, err := client.InteractNPC434(ctx, c.Username, auth.SessionKey(), realm, name, instance, factions, attempt)
	if err != nil {
		return err
	}
	fmt.Printf("Fresh server position=(%.6f, %.6f, %.6f) map=%d; final NPC distance=%.6f\n", result.World.Login.X, result.World.Login.Y, result.World.Login.Z, result.World.Login.Map, result.Distance)
	fmt.Printf("Gossip response=0x2035 GUID=%016X menu=%d text=%d options=%d quest entries=%d\n", result.Gossip.GUID, result.Gossip.Menu, result.Gossip.Text, len(result.Gossip.Options), len(result.Gossip.Quests))
	fmt.Printf("Interaction opcodes=%04X; target cleared=%t; time sync=%d\n", result.Sent, result.Target == 0, result.World.Login.TimeSyncCounter)
	fmt.Println("PASS: MMap approach independently verified, intended NPC gossip response decoded, target cleared and connections closed. No quest selection, combat, vendor transaction or spell sent.")
	return nil
}
