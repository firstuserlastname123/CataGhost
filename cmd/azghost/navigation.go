package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/azerothcore/AzerothGhost/client"
	"github.com/azerothcore/AzerothGhost/config"
	"github.com/azerothcore/AzerothGhost/pathfinding"
)

func runNavigation(c config.CLIConfig, realmName, worldAddress, name, instance string) error {
	if realmName == "" || worldAddress == "" || name == "" || instance == "" || c.DataDir == "" || c.Username == "" || c.Password == "" {
		return fmt.Errorf("navigation requires explicit realm/world/character/instance/data-dir and credentials")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	login := func() (*client.AuthClient, client.RealmInfo, error) {
		a := client.NewAuthClient(c.Username, c.Password)
		realms, err := a.Authenticate(c.AuthServer)
		if err != nil {
			return nil, client.RealmInfo{}, err
		}
		r, err := selectWorldAuthRealm(realms, realmName, worldAddress)
		return a, r, err
	}
	fmt.Println("=== CataGhost MMap Navigation (4.3.4 / 15595) ===")
	a, realm, err := login()
	if err != nil {
		return err
	}
	fmt.Printf("Realm %q ID=%d world=%s; ground-only TCPP v14 MMaps\n", realm.Name, realm.ID, realm.Address)
	finder := pathfinding.NewCataclysm434Navigator(filepath.Join(c.DataDir, "mmaps"))
	attempt, err := client.NavigateCharacter434(ctx, c.Username, a.SessionKey(), realm, name, instance, finder)
	if err != nil {
		fmt.Printf("Incomplete navigation: executed=%d movement/control opcodes=%04X\n", attempt.Executed, attempt.Sent)
		return err
	}
	p := attempt.Initial.Position
	d := attempt.Requested
	stop := attempt.Commanded
	fmt.Printf("Enumerated %q GUID=%016X map=%d initial XYZ/O=(%.3f, %.3f, %.3f, %.3f)\n", attempt.World.Login.Character.Name, attempt.Initial.GUID, attempt.World.Login.Map, p.X, p.Y, p.Z, p.Orientation)
	fmt.Printf("Requested destination=(%.3f, %.3f, %.3f)\n", d.X, d.Y, d.Z)
	fmt.Printf("Raw MMap route: type=0x%X points=%d length=%.6f\n", attempt.RawPath.Type, len(attempt.RawPath.Points), attempt.RawPath.PathLength())
	for i, p := range attempt.RawPath.Points {
		fmt.Printf("path[%d]=(%.6f, %.6f, %.6f)\n", i, p.X, p.Y, p.Z)
	}
	fmt.Printf("Executed segments=%d (at most one unit each); opcodes=%04X\n", attempt.Executed, attempt.Sent)
	fmt.Printf("Final stop command=(%.6f, %.6f, %.6f, %.6f); time-sync counter=%d\n", stop.X, stop.Y, stop.Z, stop.Orientation, attempt.World.Login.TimeSyncCounter)
	fmt.Println("Navigation session closed cleanly. Waiting 65 seconds for TCPP cleanup before independent server verification.")
	timer := time.NewTimer(65 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	a, realm, err = login()
	if err != nil {
		return err
	}
	final, err := client.ObserveWorld434(ctx, c.Username, a.SessionKey(), realm, name, instance)
	if err != nil {
		return err
	}
	m, errorToDestination, err := client.ValidateNavigationProof434(attempt, final)
	if err != nil {
		return err
	}
	f := m.Position
	fmt.Printf("Fresh LOGIN_VERIFY_WORLD/player snapshot=(%.6f, %.6f, %.6f, %.6f), map=%d GUID=%016X flags=0x%X flags2=0x%X moving=%t\n", f.X, f.Y, f.Z, f.Orientation, final.Login.Map, m.GUID, m.Flags, m.Flags2, m.Moving())
	fmt.Printf("Destination error=%.6f (tolerance 0.750); stop error verified <=0.050; time-sync=%d\n", errorToDestination, final.Login.TimeSyncCounter)
	fmt.Printf("Object updates: navigation=%d verification=%d; world verification 0x2005 and time sync 0x3CA4 processed in both sessions\n", attempt.World.Opcodes[0x4715], final.Opcodes[0x4715])
	fmt.Println("PASS: real MMap route executed through Cataclysm movement and independently confirmed by fresh server state. Clean shutdown; no teleport or unrelated gameplay commands.")
	return nil
}
