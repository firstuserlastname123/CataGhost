package main

import (
	"context"
	"fmt"

	"github.com/azerothcore/AzerothGhost/client"
	"github.com/azerothcore/AzerothGhost/config"
)

func runCharacterLogin(c config.CLIConfig, realmName, worldAddress, name, instanceAddress string) error {
	if realmName == "" || worldAddress == "" || name == "" || instanceAddress == "" {
		return fmt.Errorf("char-login requires -realm-name, -expected-world-address, -login-character and -expected-instance-address")
	}
	if c.Username == "" || c.Password == "" {
		return fmt.Errorf("char-login requires account credentials")
	}
	fmt.Println("=== CataGhost Character Login (4.3.4 / 15595) ===")
	auth := client.NewAuthClient(c.Username, c.Password)
	realms, err := auth.Authenticate(c.AuthServer)
	if err != nil {
		return err
	}
	realm, err := selectWorldAuthRealm(realms, realmName, worldAddress)
	if err != nil {
		return err
	}
	fmt.Printf("Authserver authentication succeeded. Selected realm %q (ID %d) -> %s\n", realm.Name, realm.ID, realm.Address)
	fmt.Printf("Requesting existing character %q; expected instance %s\n", name, instanceAddress)
	result, err := client.LoginCharacter434(context.Background(), c.Username, auth.SessionKey(), realm, name, instanceAddress)
	if err != nil {
		return err
	}
	fmt.Printf("World authentication and enumeration succeeded. Selected %q GUID=%016X\n", result.Character.Name, result.Character.GUID)
	fmt.Printf("Instance authenticated and resumed: %s\n", result.InstanceAddress)
	fmt.Printf("LOGIN_VERIFY_WORLD: map=%d position=(%.3f, %.3f, %.3f) orientation=%.3f\n", result.Map, result.X, result.Y, result.Z, result.Orientation)
	fmt.Printf("Initial TIME_SYNC_REQ received and acknowledged: counter=%d\n", result.TimeSyncCounter)
	fmt.Println("Character login succeeded; both connections closed. No movement or gameplay initiated.")
	return nil
}
