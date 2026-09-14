package client

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/azerothcore/AzerothGhost/pathfinding"
)

func TestObjectiveScoutPositionSelection(t *testing.T) {
	c, p, target := combatFixture434(t)
	target.Position.X = p.Position.X + 35
	finder := routeFunc434(func(_ uint32, start, dest pathfinding.Point3D) (*pathfinding.PathResult, error) {
		return navPath434(start, dest), nil
	})
	points, err := SelectObjectiveScoutPositions434(c.result.Observation, finder, c.factions, nil)
	if err != nil || len(points) != 4 {
		t.Fatalf("points=%+v err=%v", points, err)
	}
	for i, point := range points {
		if point.Order != i || point.RouteLength <= 0 {
			t.Fatalf("nondeterministic order: %+v", points)
		}
	}
	points, err = SelectObjectiveScoutPositions434(c.result.Observation, finder, c.factions, []Position434{points[0].Position})
	if err != nil || len(points) != 3 || points[0].Order != 1 {
		t.Fatalf("attempted point not excluded: %+v %v", points, err)
	}
}

func TestObjectiveScoutRejectsRouteAndHostileProximity(t *testing.T) {
	c, p, target := combatFixture434(t)
	target.Position.X = p.Position.X + 12
	valid := routeFunc434(func(_ uint32, start, dest pathfinding.Point3D) (*pathfinding.PathResult, error) {
		return navPath434(start, dest), nil
	})
	points, err := SelectObjectiveScoutPositions434(c.result.Observation, valid, c.factions, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range points {
		if point.Order == 0 {
			t.Fatalf("accepted scout endpoint on hostile: %+v", point)
		}
	}
	bad := routeFunc434(func(uint32, pathfinding.Point3D, pathfinding.Point3D) (*pathfinding.PathResult, error) {
		return nil, errors.New("no mmap route")
	})
	if points, err := SelectObjectiveScoutPositions434(c.result.Observation, bad, c.factions, nil); err != nil || len(points) != 0 {
		t.Fatalf("route-invalid point accepted: %+v %v", points, err)
	}
}

func TestObjectiveScoutRescansWithoutCombat(t *testing.T) {
	c, p, target := combatFixture434(t)
	target.Position.X = p.Position.X + 20
	neighbor := *target
	neighbor.GUID++
	neighbor.Position = &Position434{X: target.Position.X + 5, Y: target.Position.Y, Z: target.Position.Z}
	c.result.Observation.World.Store.objects[neighbor.GUID] = &neighbor
	observations := 0
	moves := 0
	finder := routeFunc434(func(_ uint32, start, dest pathfinding.Point3D) (*pathfinding.PathResult, error) {
		return navPath434(start, dest), nil
	})
	result, err := ScoutObjective434(context.Background(), finder, c.factions,
		func(context.Context) (QuestProgressObservation434, error) {
			observations++
			if observations == 2 {
				delete(c.result.Observation.World.Store.objects, neighbor.GUID)
			}
			return c.result.Observation, nil
		},
		func(_ context.Context, position Position434) error {
			moves++
			*p.Position = position
			return nil
		})
	if err != nil || result.Candidate == nil || observations != 2 || moves != 1 || len(result.Visited) != 1 {
		t.Fatalf("result=%+v observations=%d moves=%d err=%v", result, observations, moves, err)
	}
	// The scouting API exposes no packet sender, selection, or attack callback;
	// reaching a candidate therefore cannot begin combat or choose a second target.
	if result.Candidate.Candidate.GUID != target.GUID {
		t.Fatalf("unexpected target %+v", result.Candidate)
	}
}

func TestObjectiveScoutBoundedExhaustion(t *testing.T) {
	c, p, target := combatFixture434(t)
	target.Position.X = p.Position.X + 35
	observations, moves := 0, 0
	finder := routeFunc434(func(_ uint32, start, dest pathfinding.Point3D) (*pathfinding.PathResult, error) {
		// Objective approaches are longer than scout legs and remain invalid.
		if distance434(start, dest) > 16 {
			return nil, errors.New("objective route unavailable")
		}
		return navPath434(start, dest), nil
	})
	_, err := ScoutObjective434(context.Background(), finder, c.factions,
		func(context.Context) (QuestProgressObservation434, error) {
			observations++
			return c.result.Observation, nil
		},
		func(_ context.Context, position Position434) error {
			moves++
			*p.Position = position
			// Keep the unsafe fixture at a stable relative distance.
			target.Position.X, target.Position.Y = p.Position.X+35, p.Position.Y
			return nil
		})
	if err == nil || !strings.Contains(err.Error(), "BAD_TEST") || moves > maxObjectiveScouts434 || observations > maxObjectiveScouts434+1 {
		t.Fatalf("observations=%d moves=%d err=%v", observations, moves, err)
	}
}
