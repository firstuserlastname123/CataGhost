package client

import (
	"errors"
	"github.com/azerothcore/AzerothGhost/pathfinding"
	"testing"
)

func TestObjective434Isolation(t *testing.T) {
	for _, tc := range []struct {
		name              string
		x, y              float32
		routeBad, unknown bool
		want              bool
	}{
		{name: "isolated", x: 80, want: true},
		{name: "near player", x: -5},
		{name: "near target", x: 28},
		{name: "near path", x: 15, y: 2},
		{name: "unknown faction", x: 12, unknown: true},
		{name: "invalid route", x: 80, routeBad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, p, n := combatFixture434(t)
			n.Position = &Position434{X: p.Position.X + 30, Y: p.Position.Y, Z: p.Position.Z}
			other := *n
			other.GUID++
			other.Position = &Position434{X: p.Position.X + tc.x, Y: p.Position.Y + tc.y, Z: p.Position.Z}
			other.Fields = map[uint16]uint32{FieldEntry434: 999, FieldHealth434: 42, FieldMaxHealth434: 42, FieldFaction434: 2}
			if tc.unknown {
				other.Fields[FieldFaction434] = 9999
			}
			c.result.Observation.World.Store.objects[other.GUID] = &other
			finder := routeFunc434(func(_ uint32, s, d pathfinding.Point3D) (*pathfinding.PathResult, error) {
				if tc.routeBad {
					return nil, errors.New("route unavailable")
				}
				return navPath434(s, d), nil
			})
			got, err := InspectObjectiveIsolation434(c.result.Observation, finder, c.factions)
			if err != nil || len(got) != 1 || got[0].Eligible != tc.want {
				t.Fatalf("got %+v err=%v", got, err)
			}
			if len(got[0].Neighbors) != 1 {
				t.Fatal("neighbor evidence missing")
			}
		})
	}
}

func TestObjective434IsolationSegment(t *testing.T) {
	a, b := pathfinding.Point3D{}, pathfinding.Point3D{X: 40}
	if d := segmentDistance434(pathfinding.Point3D{X: 20, Y: 3}, a, b); d != 3 {
		t.Fatal(d)
	}
	if d := segmentDistance434(pathfinding.Point3D{X: -4}, a, b); d != 4 {
		t.Fatal(d)
	}
	if d := segmentDistance434(pathfinding.Point3D{X: 4}, a, a); d != 4 {
		t.Fatal(d)
	}
}
