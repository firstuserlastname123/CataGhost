package main

import (
	"testing"

	"github.com/azerothcore/AzerothGhost/config"
)

func TestCharacterEnumRequiresSelectionAndCredentials(t *testing.T) {
	for _, tc := range []struct{ name, address string }{{"", ""}, {"Test", ""}, {"", "localhost:9000"}, {"Test", "localhost:9000"}} {
		if runCharacterEnum(config.CLIConfig{}, tc.name, tc.address) == nil {
			t.Fatal("missing selectors or credentials accepted")
		}
	}
}
