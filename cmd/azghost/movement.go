package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"time"

	"github.com/azerothcore/AzerothGhost/client"
	"github.com/azerothcore/AzerothGhost/config"
)

func runMovement(c config.CLIConfig, realmName, worldAddress, name, instanceAddress string) error {
	if realmName == "" || worldAddress == "" || name == "" || instanceAddress == "" || c.Username == "" || c.Password == "" {
		return fmt.Errorf("movement requires explicit realm/world/character/instance and account credentials")
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
	fmt.Println("=== CataGhost Controlled Movement (4.3.4 / 15595) ===")
	a, realm, err := login()
	if err != nil {
		return err
	}
	fmt.Printf("Authenticated. Realm %q ID=%d world=%s\n", realm.Name, realm.ID, realm.Address)
	attempt, err := client.MoveCharacter434(ctx, c.Username, a.SessionKey(), realm, name, instanceAddress)
	if err != nil {
		return err
	}
	p := attempt.Initial.Position
	q := attempt.Stop.Position
	fmt.Printf("Selected %q through enumeration; GUID=%016X; map=%d; time-sync=%d\n", attempt.World.Login.Character.Name, attempt.Initial.GUID, attempt.World.Login.Map, attempt.World.Login.TimeSyncCounter)
	fmt.Printf("Initial server XYZ/O=(%.3f, %.3f, %.3f, %.3f) flags=0x%08X flags2=0x%03X timestamp=%d run-speed=%.3f\n", p.X, p.Y, p.Z, p.Orientation, attempt.Initial.Flags, attempt.Initial.Flags2, attempt.Initial.Timestamp, attempt.Initial.RunSpeed)
	fmt.Printf("Stop proposal XYZ/O=(%.3f, %.3f, %.3f, %.3f) flags=0x%08X timestamp=%d\n", q.X, q.Y, q.Z, q.Orientation, attempt.Stop.Flags, attempt.Stop.Timestamp)
	fmt.Printf("Sent movement/control opcodes: %04X; both sockets closed cleanly. Server proof pending.\n", attempt.Sent)
	// WorldSession::expireTime is 60000ms on socket loss. Wait for ordinary
	// server cleanup/save; no DB access, logout extension, or forced disconnect.
	fmt.Println("Waiting 65 seconds for TCPP disconnect cleanup before a fresh read-only login snapshot.")
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
	final, err := client.ObserveWorld434(ctx, c.Username, a.SessionKey(), realm, name, instanceAddress)
	if err != nil {
		return err
	}
	proof, err := client.ValidateMovementProof434(attempt, final)
	if err != nil {
		return err
	}
	f := proof.Position
	fmt.Printf("Fresh server LOGIN_VERIFY_WORLD and player create: XYZ/O=(%.3f, %.3f, %.3f, %.3f) map=%d GUID=%016X\n", f.X, f.Y, f.Z, f.Orientation, final.Login.Map, proof.GUID)
	fmt.Printf("Delta=(%.3f, %.3f, %.3f); final flags=0x%08X flags2=0x%03X moving=%t timestamp=%d\n", f.X-p.X, f.Y-p.Y, f.Z-p.Z, proof.Flags, proof.Flags2, proof.Moving(), proof.Timestamp)
	for _, phase := range []struct {
		name  string
		state client.WorldState434Result
	}{{"movement session", attempt.World}, {"verification session", final}} {
		ops := make([]int, 0, len(phase.state.Opcodes))
		for op := range phase.state.Opcodes {
			ops = append(ops, int(op))
		}
		sort.Ints(ops)
		fmt.Printf("%s server opcodes:", phase.name)
		for _, op := range ops {
			fmt.Printf(" 0x%04X:%d", op, phase.state.Opcodes[uint16(op)])
		}
		fmt.Println()
	}
	fmt.Println("PASS: server saved and reported the stop-only final position; same map, stopped state, required time sync and clean shutdown. No unrelated gameplay command sent.")
	return nil
}
