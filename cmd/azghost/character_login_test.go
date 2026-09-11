package main

import (
	"testing"

	"github.com/azerothcore/AzerothGhost/config"
)

func TestCharacterLoginRequiresSelection(t *testing.T) {
	if runCharacterLogin(config.CLIConfig{}, "", "", "", "") == nil {
		t.Fatal("accepted missing explicit selection")
	}
	if runCharacterLogin(config.CLIConfig{}, "Test", "localhost:9000", "Ghost", "localhost:9001") == nil {
		t.Fatal("accepted missing credentials")
	}
}
