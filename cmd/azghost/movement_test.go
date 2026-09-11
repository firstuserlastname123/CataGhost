package main

import (
	"github.com/azerothcore/AzerothGhost/config"
	"testing"
)

func TestMovementRequiresSelection(t *testing.T) {
	if runMovement(config.CLIConfig{}, "", "", "", "") == nil {
		t.Fatal("missing selection")
	}
}
