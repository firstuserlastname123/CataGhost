package client

import (
	"fmt"
	"math"
	"sort"

	"github.com/azerothcore/AzerothGhost/pathfinding"
)

// Isolation is a read-only environmental preflight.
// Historical QA research reports a 15-yard spawn-centered wander radius;
// configured assistance is 10 yards. The 30-yard snapshot clearance is a
// rejection heuristic, not a bound on future movement from current positions.
// Eligibility here cannot authorize combat without continuous monitoring.
const objectiveIsolationRadius434 = 30.0

type ObjectiveNeighbor434 struct {
	GUID                                         uint64
	Entry                                        uint32
	Reaction                                     string
	PlayerDistance, TargetDistance, PathDistance float64
}

type ObjectiveIsolation434 struct {
	Candidate        ObjectiveCandidate434
	Neighbors        []ObjectiveNeighbor434
	RouteError       string
	MinimumClearance float64
	RouteLength      float64
	Eligible         bool
}

func segmentDistance434(p, a, b pathfinding.Point3D) float64 {
	dx, dy, dz := float64(b.X-a.X), float64(b.Y-a.Y), float64(b.Z-a.Z)
	den := dx*dx + dy*dy + dz*dz
	t := 0.0
	if den > 0 {
		t = (float64(p.X-a.X)*dx + float64(p.Y-a.Y)*dy + float64(p.Z-a.Z)*dz) / den
		t = math.Max(0, math.Min(1, t))
	}
	x, y, z := float64(p.X-a.X)-t*dx, float64(p.Y-a.Y)-t*dy, float64(p.Z-a.Z)-t*dz
	return math.Sqrt(x*x + y*y + z*z)
}

// InspectObjectiveIsolation434 does not send packets or change a safety predicate.
// A caller must still reject controlled actors and continuously monitor any
// later movement/combat; a stationary snapshot cannot guarantee future isolation.
func InspectObjectiveIsolation434(observation QuestProgressObservation434, finder RouteFinder434, factions *NPCFactions434) ([]ObjectiveIsolation434, error) {
	if err := VerifyUncontrolledPlayer434(&observation.World.Store); err != nil {
		return nil, err
	}
	objective, err := singleObjective434(observation.Quest, 0)
	if err != nil {
		return nil, err
	}
	return inspectCombatIsolation434(observation.World, finder, factions, objective.Target, false)
}

// InspectSingleCombatTargets434 applies the objective combat safety model to
// every ordinary attackable creature in a live world snapshot. Unlike the
// objective preflight it has no quest or creature-entry dependency.
func InspectSingleCombatTargets434(world WorldState434Result, finder RouteFinder434, factions *NPCFactions434) ([]ObjectiveIsolation434, error) {
	return inspectCombatIsolation434(world, finder, factions, 0, true)
}

func inspectCombatIsolation434(world WorldState434Result, finder RouteFinder434, factions *NPCFactions434, entry uint32, rejectServices bool) ([]ObjectiveIsolation434, error) {
	if err := VerifyUncontrolledPlayer434(&world.Store); err != nil {
		return nil, err
	}
	s := &world.Store
	p := s.objects[s.PlayerGUID]
	if p == nil || p.Position == nil || finder == nil || factions == nil {
		return nil, fmt.Errorf("UNKNOWN: isolation requires player position, factions and route finder")
	}
	var out []ObjectiveIsolation434
	for _, candidate := range objectiveCandidates434(s, factions, entry) {
		if rejectServices {
			n := s.objects[candidate.GUID]
			if n == nil || n.Fields[FieldNPCFlags434] != 0 {
				continue
			}
		}
		v := ObjectiveIsolation434{Candidate: candidate, MinimumClearance: math.MaxFloat64}
		dest, routeErr := npcDestination434(*p.Position, NPCCandidate434{Position: candidate.Position})
		var positions []Position434
		if routeErr == nil {
			var route *pathfinding.PathResult
			route, routeErr = finder.FindPath(uint32(p.Map), point434(*p.Position), point434(dest))
			if routeErr == nil {
				v.RouteLength = float64(route.PathLength())
				positions, routeErr = validateBoundedRoute434(point434(*p.Position), point434(dest), route, 40, 1, 50, 64)
			}
		}
		if routeErr != nil {
			v.RouteError = routeErr.Error()
		}
		for _, n := range s.Objects() {
			if n.GUID == candidate.GUID || n.GUID == p.GUID || n.Type != 3 || n.Map != p.Map {
				continue
			}
			if n.Position == nil || !finitePosition434(n.Position) {
				return nil, fmt.Errorf("UNKNOWN: nearby unit %016X lacks a finite position", n.GUID)
			}
			u, unitErr := combatUnit434(&n)
			if unitErr == nil && u.Dead {
				continue
			}
			reaction, attackable := factions.combatReaction434(&n, p)
			if !attackable && reaction == "friendly" {
				continue
			}
			np := point434(*n.Position)
			neighbor := ObjectiveNeighbor434{GUID: n.GUID, Entry: n.Fields[FieldEntry434], Reaction: reaction, PlayerDistance: distance434(np, point434(*p.Position)), TargetDistance: distance434(np, point434(candidate.Position)), PathDistance: -1}
			prev := point434(*p.Position)
			for _, pos := range positions {
				next := point434(pos)
				d := segmentDistance434(np, prev, next)
				if neighbor.PathDistance < 0 || d < neighbor.PathDistance {
					neighbor.PathDistance = d
				}
				prev = next
			}
			clearance := math.Min(neighbor.PlayerDistance, neighbor.TargetDistance)
			if neighbor.PathDistance >= 0 {
				clearance = math.Min(clearance, neighbor.PathDistance)
			}
			v.MinimumClearance = math.Min(v.MinimumClearance, clearance)
			v.Neighbors = append(v.Neighbors, neighbor)
		}
		v.Eligible = v.RouteError == "" && v.MinimumClearance > objectiveIsolationRadius434
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Eligible != out[j].Eligible {
			return out[i].Eligible
		}
		if out[i].MinimumClearance != out[j].MinimumClearance {
			return out[i].MinimumClearance > out[j].MinimumClearance
		}
		if out[i].RouteLength != out[j].RouteLength {
			return out[i].RouteLength < out[j].RouteLength
		}
		return out[i].Candidate.GUID < out[j].Candidate.GUID
	})
	return out, nil
}
