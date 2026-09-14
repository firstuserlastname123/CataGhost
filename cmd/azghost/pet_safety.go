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

func runPetSafety(c config.CLIConfig, realmName, world, name, instance string, id uint32) error {
	return runPetProbe(c, realmName, world, name, instance, id, false)
}

func runPetProbe(c config.CLIConfig, realmName, world, name, instance string, id uint32, readOnly bool) error {
	if c.Username == "" || c.Password == "" || realmName == "" || world == "" || name == "" || instance == "" {
		return fmt.Errorf("BAD_TEST: explicit pet QA credentials/endpoints/character required")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	a := client.NewAuthClient(c.Username, c.Password)
	realms, err := a.Authenticate(c.AuthServer)
	if err != nil {
		return fmt.Errorf("BAD_TEST: auth: %w", err)
	}
	realm, err := selectWorldAuthRealm(realms, realmName, world)
	if err != nil {
		return err
	}
	observe := client.ObservePetSafety434
	if readOnly {
		observe = client.ObservePetControlGate434
	}
	result, err := observe(ctx, c.Username, a.SessionKey(), realm, name, instance, id)
	b, jsonErr := json.Marshal(result)
	if jsonErr != nil {
		return jsonErr
	}
	fmt.Printf("PetSafety=%s\n", b)
	printQuestProgress434("Pet probe quest", result.Observation)
	if readOnly && c.DataDir != "" {
		factions, isolationErr := client.LoadNPCFactions434(filepath.Join(c.DataDir, "dbc", "enUS"))
		if isolationErr == nil {
			var isolation []client.ObjectiveIsolation434
			isolation, isolationErr = client.InspectObjectiveIsolation434(result.Observation, pathfinding.NewCataclysm434Navigator(filepath.Join(c.DataDir, "mmaps")), factions)
			if isolationErr == nil {
				data, marshalErr := json.Marshal(isolation)
				if marshalErr != nil {
					return marshalErr
				}
				fmt.Printf("ObjectiveIsolation=%s\n", data)
			}
		}
		if isolationErr != nil {
			return fmt.Errorf("UNKNOWN: isolation preflight: %w", isolationErr)
		}
	}
	if err != nil {
		return err
	}
	if readOnly {
		fmt.Println("Read-only pet-control-gate observation closed. No pet commands or combat sent; no objective milestone PASS claimed.")
		return nil
	}
	if !result.SafetyProven {
		return fmt.Errorf("UNKNOWN: missing authoritative pet safety")
	}
	fmt.Println("PASS: stationary pet-safety prerequisite verified. No combat, movement, dismissal, loot or turn-in. Clean disconnect. Objective milestone not yet passed.")
	return nil
}
