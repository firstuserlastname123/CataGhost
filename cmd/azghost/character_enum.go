package main

import (
	"context"
	"fmt"

	"github.com/azerothcore/AzerothGhost/client"
	"github.com/azerothcore/AzerothGhost/config"
)

func runCharacterEnum(c config.CLIConfig, name, address string) error {
	if name == "" || address == "" {
		return fmt.Errorf("char-enum requires -realm-name and -expected-world-address")
	}
	if c.Username == "" || c.Password == "" {
		return fmt.Errorf("char-enum requires account credentials")
	}
	fmt.Println("=== CataGhost Character Enumeration (4.3.4 / 15595) ===")
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
	roster, err := client.EnumerateCharacters434(context.Background(), c.Username, auth.SessionKey(), realm)
	if err != nil {
		return err
	}
	fmt.Printf("World authentication succeeded (AUTH_OK, unqueued). Found %d character(s):\n", len(roster.Characters))
	for _, c := range roster.Characters {
		fmt.Printf("  %q GUID=%016X level=%d race=%d class=%d gender=%d map=%d zone=%d position=(%.3f, %.3f, %.3f) guild=%016X flags=0x%08X customization=0x%08X first-login=%t\n", c.Name, c.GUID, c.Level, c.Race, c.Class, c.Gender, c.Map, c.Zone, c.X, c.Y, c.Z, c.GuildGUID, c.Flags, c.CustomizationFlags, c.FirstLogin)
	}
	fmt.Println("Character enumeration succeeded; connection closed. No character login sent.")
	return nil
}
