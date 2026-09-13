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

func waitQuestCleanup(ctx context.Context) error {
	t := time.NewTimer(65 * time.Second)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func runQuestAcceptance(c config.CLIConfig, realmName, worldAddress, name, instance string, inspectedID uint32) error {
	if c.Username == "" || c.Password == "" || c.DataDir == "" || realmName == "" || worldAddress == "" || name == "" || instance == "" {
		return fmt.Errorf("quest probe requires explicit credentials/realm/world/character/instance/data-dir")
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
	fmt.Println("=== CataGhost Quest Acceptance (4.3.4 / 15595) ===")
	a, realm, err := login()
	if err != nil {
		return err
	}
	approach, err := client.ApproachQuestNPC434(ctx, c.Username, a.SessionKey(), realm, name, instance, pathfinding.NewCataclysm434Navigator(filepath.Join(c.DataDir, "mmaps")), factions, 2079)
	if err != nil {
		fmt.Printf("Approach failed after %d segments\n", approach.Navigation.Executed)
		fmt.Printf("Geometry: initial=%+v NPC=%+v requested=%+v MMap=%+v\n", approach.Navigation.Initial.Position, approach.NPC, approach.Navigation.Requested, approach.Navigation.RawPath)
		return err
	}
	fmt.Printf("Character=%s GUID=%016X realm=%s ID=%d map=%d\n", name, approach.Navigation.Initial.GUID, realm.Name, realm.ID, approach.Navigation.World.Login.Map)
	fmt.Printf("NPC=%016X entry=%d flags=0x%X initial range=%.6f\n", approach.NPC.GUID, approach.NPC.Entry, approach.NPC.Flags, approach.NPC.Distance)
	fmt.Printf("Geometry: player=%+v NPC=%+v MMap smoothed/height-normalized points=%+v\n", approach.Navigation.Initial.Position, approach.NPC.Position, approach.Navigation.RawPath.Points)
	fmt.Printf("MMap points=%d length=%.6f segments=%d destination=%+v stop=%+v\n", len(approach.Navigation.RawPath.Points), approach.Navigation.RawPath.PathLength(), approach.Navigation.Executed, approach.Navigation.Requested, approach.Navigation.Commanded)
	fmt.Println("Approach closed; waiting 65 seconds before independent position proof and quest conversation.")
	if err := waitQuestCleanup(ctx); err != nil {
		return err
	}
	a, realm, err = login()
	if err != nil {
		return err
	}
	result, err := client.QuestConversation434(ctx, c.Username, a.SessionKey(), realm, name, instance, factions, approach, inspectedID)
	fmt.Printf("Quest selected=%d type=%d level=%d title=%q; acceptSent=%t; quest opcodes=%04X\n", result.Selected.ID, result.Selected.Type, int32(result.Selected.Level), result.Selected.Title, result.AcceptSent, result.Sent)
	fmt.Printf("Live gossip menu=%d text=%d quests=%+v; initial quest log=%+v\n", result.Interaction.Gossip.Menu, result.Interaction.Gossip.Text, result.Interaction.Gossip.Quests, result.InitialLog)
	if err != nil {
		return err
	}
	fmt.Printf("Fresh arrival=%+v distance=%.6f\n", result.Interaction.World.Login, result.Interaction.Distance)
	d := result.Details
	if d.ID == 0 {
		fmt.Println("Details query intentionally skipped for AUTO_ACCEPT offer; explicit accept uses live gossip identity.")
	}
	fmt.Printf("Gossip menu=%d text=%d; details giver=%016X ID=%d flags=0x%X range=%.6f\n", result.Interaction.Gossip.Menu, result.Interaction.Gossip.Text, d.Giver, d.ID, d.Flags, result.Interaction.Distance)
	fmt.Printf("Objectives=%q description=%q requiredSpell=%d reward choices=%d guaranteed=%d money=%d XP=%d\n", d.Objectives, d.Description, d.RequiredSpell, d.Rewards.ChoiceCount, d.Rewards.ItemCount, d.Rewards.Money, d.Rewards.XP)
	fmt.Printf("Initial quest log=%+v; post-accept=%+v\n", result.InitialLog, result.Accepted)
	if result.AcquisitionMethod == client.QuestAcquisitionAutoAccept434 {
		if result.AcceptSent || result.Accepted == nil {
			return fmt.Errorf("auto-accept acquisition proof missing")
		}
		fmt.Println("AUTO_ACCEPT acquisition observed in server quest-log fields; waiting 65 seconds for reconnect persistence proof.")
	} else if inspectedID == 0 && !result.AcceptSent {
		fmt.Println("DETAILS VERIFIED: no acceptance sent. Inspect quest suitability before rerunning with its live-discovered ID.")
		return nil
	} else if !result.AcceptSent || result.Accepted == nil {
		return fmt.Errorf("missing server acceptance evidence")
	}
	if result.AcquisitionMethod != client.QuestAcquisitionAutoAccept434 {
		fmt.Println("Server quest-log addition observed; waiting 65 seconds for cleanup before persistence proof.")
	}
	if err := waitQuestCleanup(ctx); err != nil {
		return err
	}
	a, realm, err = login()
	if err != nil {
		return err
	}
	final, err := client.ObserveWorld434(ctx, c.Username, a.SessionKey(), realm, name, instance)
	if err != nil {
		return err
	}
	slot, err := client.VerifyQuestReconnect434(result, final)
	if err != nil {
		return err
	}
	fmt.Printf("Fresh server quest log: %+v; player=%016X map=%d timeSync=%d\n", slot, final.Store.PlayerGUID, final.Login.Map, final.Login.TimeSyncCounter)
	fmt.Println("PASS: exactly one explicit acceptance, server object-field confirmation and independent reconnect persistence. No combat, loot, objective execution, completion, turn-in, vendor or spell request. Clean shutdown.")
	return nil
}

func runQuestReconnect(c config.CLIConfig, realmName, worldAddress, name, instance string, questID uint32) error {
	if questID == 0 {
		return fmt.Errorf("quest reconnect requires an explicit inspected quest ID")
	}
	if c.Username == "" || c.Password == "" || c.DataDir == "" || realmName == "" || worldAddress == "" || name == "" || instance == "" {
		return fmt.Errorf("quest reconnect requires explicit inputs")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	a := client.NewAuthClient(c.Username, c.Password)
	realms, err := a.Authenticate(c.AuthServer)
	if err != nil {
		return err
	}
	realm, err := selectWorldAuthRealm(realms, realmName, worldAddress)
	if err != nil {
		return err
	}
	s, err := client.ObserveWorld434(ctx, c.Username, a.SessionKey(), realm, name, instance)
	if err != nil {
		return err
	}
	log, err := client.QuestLog434(&s.Store)
	if err != nil {
		return err
	}
	for _, q := range log {
		if q.ID == questID {
			fmt.Printf("Quest %d persisted after reconnect: %+v; map=%d time-sync=%d\n", questID, q, s.Login.Map, s.Login.TimeSyncCounter)
			fmt.Println("PASS: server-provided quest acquisition persisted; no accept packet sent in verification.")
			return nil
		}
	}
	return fmt.Errorf("quest %d absent after reconnect; log=%+v", questID, log)
}
