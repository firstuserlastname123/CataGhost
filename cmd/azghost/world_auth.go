package main

import (
	"context"
	"fmt"
	"github.com/azerothcore/AzerothGhost/client"
	"github.com/azerothcore/AzerothGhost/config"
)

func selectWorldAuthRealm(realms []client.RealmInfo, name, address string) (client.RealmInfo, error) {
	var selected client.RealmInfo
	count := 0
	for _, realm := range realms {
		if realm.Name == name {
			selected = realm
			count++
		}
	}
	if count != 1 {
		return selected, fmt.Errorf("expected exactly one realm named %q, found %d", name, count)
	}
	if selected.Address != address {
		return selected, fmt.Errorf("selected realm address does not match expected address")
	}
	return selected, nil
}

func runWorldAuth(c config.CLIConfig, name, address string) error {
	if name == "" || address == "" {
		return fmt.Errorf("world-auth requires -realm-name and -expected-world-address")
	}
	if c.Username == "" || c.Password == "" {
		return fmt.Errorf("world-auth requires account credentials")
	}
	fmt.Println("=== CataGhost World Auth Probe (4.3.4 / 15595) ===")
	auth := client.NewAuthClient(c.Username, c.Password)
	realms, err := auth.Authenticate(c.AuthServer)
	if err != nil {
		return fmt.Errorf("authserver: %w", err)
	}
	realm, err := selectWorldAuthRealm(realms, name, address)
	if err != nil {
		return err
	}
	fmt.Printf("Selected realm %q (ID %d) -> %s\n", realm.Name, realm.ID, realm.Address)
	if err := client.AuthenticateWorld434(context.Background(), c.Username, auth.SessionKey(), realm); err != nil {
		return err
	}
	fmt.Println("World authentication succeeded (AUTH_OK, unqueued); connection closed.")
	return nil
}
