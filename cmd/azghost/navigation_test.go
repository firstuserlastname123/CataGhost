package main

import (
	"github.com/azerothcore/AzerothGhost/config"
	"testing"
)

func TestNavigationRequiresExplicitInputs(t *testing.T) {
	if runNavigation(config.CLIConfig{}, "", "", "", "") == nil {
		t.Fatal("missing inputs accepted")
	}
}
