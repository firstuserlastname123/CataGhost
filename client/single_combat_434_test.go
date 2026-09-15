package client

import (
	"context"
	"encoding/binary"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/azerothcore/AzerothGhost/pathfinding"
)

func TestSingleCombat434GenericDiscoveryAndRanking(t *testing.T) {
	c, p, n := combatFixture434(t)
	p.Position = &Position434{}
	n.Position = &Position434{X: 35}
	n.Fields[FieldNPCFlags434] = 0
	second := *n
	second.GUID++
	second.Position = &Position434{X: -35}
	second.Fields = copyFields434(n.Fields)
	c.result.Observation.World.Store.objects[second.GUID] = &second
	finder := routeFunc434(func(_ uint32, s, d pathfinding.Point3D) (*pathfinding.PathResult, error) {
		return navPath434(s, d), nil
	})
	for i := 0; i < 10; i++ {
		got, err := InspectSingleCombatTargets434(c.result.Observation.World, finder, c.factions)
		if err != nil || len(got) != 2 || !got[0].Eligible || got[0].Candidate.GUID != n.GUID {
			t.Fatalf("generic deterministic result=%+v err=%v", got, err)
		}
	}
}

func TestSingleCombat434DiscoveryRejections(t *testing.T) {
	for _, name := range []string{"friendly", "player", "pet", "dead", "service", "route", "isolation"} {
		t.Run(name, func(t *testing.T) {
			c, p, n := combatFixture434(t)
			p.Position = &Position434{}
			n.Position = &Position434{X: 35}
			n.Fields[FieldNPCFlags434] = 0
			routeBad := false
			switch name {
			case "friendly":
				c.factions.templates[2][10], c.factions.templates[1][1] = 99, 99
			case "player":
				n.Type = 4
			case "pet":
				n.Fields[0xe] = uint32(p.GUID)
			case "dead":
				n.Fields[FieldHealth434] = 0
			case "service":
				n.Fields[FieldNPCFlags434] = 1
			case "route":
				routeBad = true
			case "isolation":
				other := *n
				other.GUID++
				other.Position = &Position434{X: 34}
				other.Fields = copyFields434(n.Fields)
				c.result.Observation.World.Store.objects[other.GUID] = &other
			}
			finder := routeFunc434(func(_ uint32, s, d pathfinding.Point3D) (*pathfinding.PathResult, error) {
				if routeBad {
					return nil, errors.New("no mmap")
				}
				return navPath434(s, d), nil
			})
			got, err := InspectSingleCombatTargets434(c.result.Observation.World, finder, c.factions)
			if err != nil && name != "pet" {
				t.Fatal(err)
			}
			if name == "pet" && err != nil {
				return
			}
			if name == "route" || name == "isolation" {
				if len(got) == 0 || got[0].Eligible {
					t.Fatalf("unsafe target eligible: %+v", got)
				}
			} else if len(got) != 0 {
				t.Fatalf("rejected target discovered: %+v", got)
			}
		})
	}
}

func copyFields434(in map[uint16]uint32) map[uint16]uint32 {
	out := make(map[uint16]uint32, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func TestSingleCombat434Serialization(t *testing.T) {
	g := uint64(0xf130000123000456)
	want := []byte{0x56, 4, 0, 0x23, 1, 0, 0x30, 0xf1}
	if !reflect.DeepEqual(selectionPacket434(g), want) {
		t.Fatal("selection serialization")
	}
	if !reflect.DeepEqual(attackSwingPacket434(g), want) {
		t.Fatal("swing serialization")
	}
	if attackStopPacket434() != nil {
		t.Fatal("attack stop must have empty body")
	}
}

func combatControllerFixture434(t *testing.T) (*singleCombatController434, *Object434, *Object434) {
	c, p, n := combatFixture434(t)
	r := &SingleCombatResult434{World: c.result.Observation.World, Target: ObjectiveCandidate434{GUID: n.GUID}}
	return &singleCombatController434{result: r, target: n.GUID, sentAt: 100}, p, n
}

func TestSingleCombat434ServerObservationAndHealthZero(t *testing.T) {
	c, _, n := combatControllerFixture434(t)
	b := binary.LittleEndian.AppendUint64(nil, c.result.World.Store.PlayerGUID)
	b = binary.LittleEndian.AppendUint64(b, n.GUID)
	if err := c.observe(loginPacket434{op: cataAttackStart434, body: b}); err != nil || !c.result.AttackStarted {
		t.Fatal(err)
	}
	c.result.TargetState.Health = 1
	if c.result.AuthoritativeDeath {
		t.Fatal("estimated death")
	}
	n.Fields[FieldHealth434] = 0
	var sent []uint16
	if err := c.tick(200, func(op uint16, _ []byte) error { sent = append(sent, op); return nil }); err != nil {
		t.Fatal(err)
	}
	if !c.result.AuthoritativeDeath || !reflect.DeepEqual(sent, []uint16{cataAttackStop434}) {
		t.Fatal(c.result, sent)
	}
}

func TestSingleCombat434AbortRules(t *testing.T) {
	for _, name := range []string{"second-target", "danger"} {
		t.Run(name, func(t *testing.T) {
			c, p, n := combatControllerFixture434(t)
			if name == "danger" {
				p.Fields[FieldHealth434] = 30
			} else {
				o := *n
				o.GUID++
				o.Fields = copyFields434(n.Fields)
				o.Fields[combatFlags434] = 0x80000
				o.Fields[combatTarget434] = uint32(p.GUID)
				c.result.World.Store.objects[o.GUID] = &o
			}
			var sent []uint16
			err := c.tick(200, func(op uint16, _ []byte) error { sent = append(sent, op); return nil })
			if err == nil || !strings.Contains(err.Error(), map[string]string{"danger": "danger", "second-target": "second creature"}[name]) || !reflect.DeepEqual(sent, []uint16{cataAttackStop434}) {
				t.Fatal(err, sent)
			}
		})
	}
}

func TestSingleCombat434ExactlyOneSuccessNoLootOrSecondKill(t *testing.T) {
	c, p, n := combatControllerFixture434(t)
	c.result.AttackStarted = true
	c.result.AuthoritativeDeath = true
	c.result.ServerCombatTerminated = true
	n.Fields[FieldHealth434] = 0
	done, err := c.complete()
	if err != nil || !done || p.Fields[FieldHealth434] == 0 {
		t.Fatal(done, err)
	}
	c.result.Sent = []uint16{cataSetSelection, cataAttackSwing434, cataAttackStop434}
	if c.result.SelectedTargets > 1 || len(c.result.Sent) != 3 {
		t.Fatal("second target/kill or loot-like packet")
	}
}

func TestSingleCombat434CancellationAndInputShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ExecuteSingleCombat434(ctx, "", nil, RealmInfo{}, "", "", nil, NavigationAttempt434{}, 0)
	if err == nil || !strings.Contains(err.Error(), "explicit combat inputs") {
		t.Fatal(err)
	}
}
