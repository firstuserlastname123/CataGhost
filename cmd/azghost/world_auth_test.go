package main

import (
	"github.com/azerothcore/AzerothGhost/client"
	"github.com/azerothcore/AzerothGhost/config"
	"testing"
)

func TestWorldAuthRealmSelection(t *testing.T) {
	realms := []client.RealmInfo{{Name: "Test Realm", Address: "localhost:9000", ID: 73}}
	got, err := selectWorldAuthRealm(realms, "Test Realm", "localhost:9000")
	if err != nil || got.ID != 73 {
		t.Fatalf("selection %v %v", got, err)
	}
	if _, err := selectWorldAuthRealm(realms, "Missing", "localhost:9000"); err == nil {
		t.Fatal("missing realm accepted")
	}
	if _, err := selectWorldAuthRealm(realms, "Test Realm", "localhost:9001"); err == nil {
		t.Fatal("wrong address accepted")
	}
	if _, err := selectWorldAuthRealm(append(realms, realms[0]), "Test Realm", "localhost:9000"); err == nil {
		t.Fatal("ambiguous realm accepted")
	}
}

func TestWorldAuthRequiresExplicitSelection(t *testing.T) {
	if runWorldAuth(config.CLIConfig{}, "", "") == nil {
		t.Fatal("missing selector accepted")
	}
}
