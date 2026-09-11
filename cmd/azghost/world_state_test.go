package main

import (
	"github.com/azerothcore/AzerothGhost/config"
	"testing"
)

func TestWorldStateRequiresSelection(t *testing.T) {
	if runWorldState(config.CLIConfig{}, "", "", "", "") == nil {
		t.Fatal("missing selection accepted")
	}
	if runWorldState(config.CLIConfig{}, "Synthetic", "localhost:9000", "Synthetic", "localhost:9001") == nil {
		t.Fatal("missing credentials accepted")
	}
}
