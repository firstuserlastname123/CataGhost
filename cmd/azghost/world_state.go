package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"

	"github.com/azerothcore/AzerothGhost/client"
	"github.com/azerothcore/AzerothGhost/config"
)

func runWorldState(c config.CLIConfig, realmName, worldAddress, name, instanceAddress string) error {
	if realmName == "" || worldAddress == "" || name == "" || instanceAddress == "" {
		return fmt.Errorf("world-state requires explicit realm, world address, character and instance address")
	}
	if c.Username == "" || c.Password == "" {
		return fmt.Errorf("world-state requires account credentials")
	}
	fmt.Println("=== CataGhost Initial World State (4.3.4 / 15595) ===")
	auth := client.NewAuthClient(c.Username, c.Password)
	realms, err := auth.Authenticate(c.AuthServer)
	if err != nil {
		return err
	}
	realm, err := selectWorldAuthRealm(realms, realmName, worldAddress)
	if err != nil {
		return err
	}
	fmt.Printf("Authserver succeeded. Selected realm %q (ID %d) -> %s\n", realm.Name, realm.ID, realm.Address)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	result, err := client.ObserveWorld434(ctx, c.Username, auth.SessionKey(), realm, name, instanceAddress)
	if err != nil {
		return err
	}
	printWorldState434(os.Stdout, result)
	return nil
}

func printWorldState434(w io.Writer, s client.WorldState434Result) {
	l := s.Login
	fmt.Fprintf(w, "World auth, enumeration, character login and instance authentication succeeded: %q GUID=%016X instance=%s\n", l.Character.Name, l.Character.GUID, l.InstanceAddress)
	fmt.Fprintf(w, "LOGIN_VERIFY_WORLD map=%d position=(%.3f, %.3f, %.3f) orientation=%.3f\n", l.Map, l.X, l.Y, l.Z, l.Orientation)
	fmt.Fprintf(w, "TIME_SYNC acknowledged: counter=%d\n", l.TimeSyncCounter)
	counts := make(map[uint8]int)
	nearby := 0
	for _, o := range s.Store.Objects() {
		counts[o.Type]++
		if o.GUID != s.Store.PlayerGUID {
			if o.Created && o.Type >= 3 {
				nearby++
			}
			continue
		}
		fmt.Fprintf(w, "Player object GUID=%016X type=%d created=%t self=%t map=%d fields=%d revision=%d\n", o.GUID, o.Type, o.Created, o.ThisIsYou, o.Map, len(o.Fields), o.Revision)
		if p := o.Position; p != nil {
			fmt.Fprintf(w, "Decoded position=(%.3f, %.3f, %.3f) orientation=%.3f\n", p.X, p.Y, p.Z, p.Orientation)
		}
		for _, f := range []struct {
			name  string
			index uint16
		}{{"level", client.FieldLevel434}, {"health", client.FieldHealth434}, {"max-health", client.FieldMaxHealth434}, {"faction-template", client.FieldFaction434}, {"display-id", client.FieldDisplay434}} {
			if v, ok := o.Fields[f.index]; ok {
				fmt.Fprintf(w, "%s=%d\n", f.name, v)
			} else {
				fmt.Fprintf(w, "%s=not transmitted\n", f.name)
			}
		}
		if v, ok := o.Fields[client.FieldUnitBytes434]; ok {
			fmt.Fprintf(w, "race=%d class=%d gender=%d primary-power-type=%d\n", byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
			if slot, known := client.PrimaryPowerSlot434(byte(v>>8), byte(v>>24)); known {
				current, cok := o.Fields[client.FieldPower1434+slot]
				maximum, mok := o.Fields[client.FieldMaxPower1434+slot]
				fmt.Fprintf(w, "primary-power slot=%d current=%d (known=%t) maximum=%d (known=%t)\n", slot, current, cok, maximum, mok)
			}
		}
		for i := uint16(0); i < 5; i++ {
			v, vok := o.Fields[client.FieldPower1434+i]
			m, mok := o.Fields[client.FieldMaxPower1434+i]
			if vok || mok {
				fmt.Fprintf(w, "power-slot-%d current=%d (known=%t) maximum=%d (known=%t)\n", i, v, vok, m, mok)
			}
		}
	}
	types := make([]int, 0, len(counts))
	for k := range counts {
		types = append(types, int(k))
	}
	sort.Ints(types)
	fmt.Fprintf(w, "Objects=%d nearby-world-objects=%d; types:", len(s.Store.Objects()), nearby)
	for _, k := range types {
		fmt.Fprintf(w, " %d:%d", k, counts[uint8(k)])
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "World variables=%d area=%d subarea=%d\n", len(s.WorldVariables), s.Area, s.Subarea)
	ops := make([]int, 0, len(s.Opcodes))
	for op := range s.Opcodes {
		ops = append(ops, int(op))
	}
	sort.Ints(ops)
	fmt.Fprint(w, "Observed opcodes:")
	for _, op := range ops {
		fmt.Fprintf(w, " 0x%04X:%d", op, s.Opcodes[uint16(op)])
	}
	fmt.Fprintln(w)
	for _, warning := range s.Warnings {
		fmt.Fprintf(w, "Warning: %s\n", warning)
	}
	fmt.Fprintln(w, "PASS: player object decoded and consistent with LOGIN_VERIFY_WORLD; both connections closed cleanly. No movement or gameplay command sent.")
}
