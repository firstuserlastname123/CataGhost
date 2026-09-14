package client

import (
	"context"
	"fmt"
	"math"
)

const (
	maxObjectiveScouts434      = 4
	objectiveScoutDistance434  = 12.0
	objectiveScoutClearance434 = 10.0
	objectiveScoutArrival434   = 1.0
)

// ObjectiveScoutPosition434 is a deterministic, route-validated observation
// point. Clearance is measured after the first four yards of departure: a bot
// cannot retroactively make its current position safe, but it must not route
// closer to known relevant units and its endpoint must retain assistance-range
// clearance. Candidate combat still uses the stricter 30-yard snapshot model.
type ObjectiveScoutPosition434 struct {
	Position         Position434
	Order            int
	RouteLength      float64
	MinimumClearance float64
}

func samePosition434(a, b Position434) bool {
	return distance434(point434(a), point434(b)) < objectiveScoutArrival434
}

// SelectObjectiveScoutPositions434 returns at most four safe points in stable
// forward, left, right, back order. It sends no packets and starts no combat.
func SelectObjectiveScoutPositions434(o QuestProgressObservation434, finder RouteFinder434, factions *NPCFactions434, attempted []Position434) ([]ObjectiveScoutPosition434, error) {
	if _, err := singleObjective434(o.Quest, 0); err != nil {
		return nil, err
	}
	if err := VerifyUncontrolledPlayer434(&o.World.Store); err != nil {
		return nil, err
	}
	s := &o.World.Store
	p := s.objects[s.PlayerGUID]
	if p == nil || p.Position == nil || finder == nil || factions == nil {
		return nil, fmt.Errorf("UNKNOWN: scouting requires player position, factions and route finder")
	}
	angles := [...]float64{0, math.Pi / 2, -math.Pi / 2, math.Pi}
	var out []ObjectiveScoutPosition434
	for order, offset := range angles {
		a := float64(p.Position.Orientation) + offset
		dest := Position434{X: p.Position.X + float32(objectiveScoutDistance434*math.Cos(a)), Y: p.Position.Y + float32(objectiveScoutDistance434*math.Sin(a)), Z: p.Position.Z, Orientation: float32(a)}
		skip := false
		for _, old := range attempted {
			skip = skip || samePosition434(dest, old)
		}
		if skip {
			continue
		}
		route, err := finder.FindPath(uint32(p.Map), point434(*p.Position), point434(dest))
		if err != nil {
			continue
		}
		segments, err := validateBoundedRoute434(point434(*p.Position), point434(dest), route, 16, 2, 24, 32)
		if err != nil {
			continue
		}
		minimum := math.MaxFloat64
		endpointSafe := true
		for _, n := range s.Objects() {
			if n.GUID == p.GUID || n.Type != 3 || n.Map != p.Map || n.Position == nil {
				continue
			}
			u, unitErr := combatUnit434(&n)
			if unitErr == nil && u.Dead {
				continue
			}
			_, relevant := factions.combatReaction434(&n, p)
			if !relevant {
				continue
			}
			np := point434(*n.Position)
			if d := distance434(np, point434(dest)); d <= objectiveScoutClearance434 {
				endpointSafe = false
			}
			for _, sample := range segments {
				if distance434(point434(sample), point434(*p.Position)) < 4 {
					continue
				}
				minimum = math.Min(minimum, distance434(np, point434(sample)))
			}
		}
		if !endpointSafe || minimum <= objectiveScoutClearance434 {
			continue
		}
		out = append(out, ObjectiveScoutPosition434{Position: dest, Order: order, RouteLength: float64(route.PathLength()), MinimumClearance: minimum})
	}
	if len(out) > maxObjectiveScouts434 {
		out = out[:maxObjectiveScouts434]
	}
	return out, nil
}

type ObjectiveScoutResult434 struct {
	Observation QuestProgressObservation434
	Candidate   *ObjectiveIsolation434
	Visited     []ObjectiveScoutPosition434
	Rescans     int
}

type ObjectiveScoutObserve434 func(context.Context) (QuestProgressObservation434, error)
type ObjectiveScoutMove434 func(context.Context, Position434) error

// ScoutObjective434 rescans after every completed movement and exhausts a
// strict movement budget. The move callback is observation movement only;
// target selection and attack opcodes are intentionally absent from this API.
func ScoutObjective434(ctx context.Context, finder RouteFinder434, factions *NPCFactions434, observe ObjectiveScoutObserve434, move ObjectiveScoutMove434) (ObjectiveScoutResult434, error) {
	var result ObjectiveScoutResult434
	if observe == nil || move == nil {
		return result, fmt.Errorf("BAD_TEST: scouting callbacks required")
	}
	var attempted []Position434
	for scan := 0; scan <= maxObjectiveScouts434; scan++ {
		o, err := observe(ctx)
		if err != nil {
			return result, err
		}
		result.Observation, result.Rescans = o, scan
		checks, err := InspectObjectiveIsolation434(o, finder, factions)
		if err != nil {
			return result, err
		}
		if len(checks) > 0 && checks[0].Eligible {
			candidate := checks[0]
			result.Candidate = &candidate
			return result, nil
		}
		if scan == maxObjectiveScouts434 {
			break
		}
		points, err := SelectObjectiveScoutPositions434(o, finder, factions, attempted)
		if err != nil {
			return result, err
		}
		if len(points) == 0 {
			break
		}
		chosen := points[0]
		if err := move(ctx, chosen.Position); err != nil {
			return result, err
		}
		attempted = append(attempted, chosen.Position)
		result.Visited = append(result.Visited, chosen)
	}
	return result, fmt.Errorf("BAD_TEST: bounded scouting exhausted; no route-valid isolated objective candidate; no combat sent")
}
