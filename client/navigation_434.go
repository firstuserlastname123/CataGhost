package client

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/azerothcore/AzerothGhost/pathfinding"
)

// RouteFinder434 preserves the inherited PathType so shortcuts, partial routes
// and missing-tile straight-line fallbacks cannot masquerade as MMap routes.
type RouteFinder434 interface {
	FindPath(uint32, pathfinding.Point3D, pathfinding.Point3D) (*pathfinding.PathResult, error)
}
type NavigationAttempt434 struct {
	World     WorldState434Result
	Initial   Movement434
	Requested pathfinding.Point3D
	RawPath   pathfinding.PathResult
	Segments  []Position434
	Executed  int
	Commanded Position434
	Sent      []uint16
}

func point434(p Position434) pathfinding.Point3D { return pathfinding.Point3D{X: p.X, Y: p.Y, Z: p.Z} }
func distance434(a, b pathfinding.Point3D) float64 {
	dx, dy, dz := float64(a.X-b.X), float64(a.Y-b.Y), float64(a.Z-b.Z)
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func validateRoute434(start, dest pathfinding.Point3D, r *pathfinding.PathResult) ([]Position434, error) {
	for _, p := range []pathfinding.Point3D{start, dest} {
		if !finitePosition434(&Position434{X: p.X, Y: p.Y, Z: p.Z}) {
			return nil, fmt.Errorf("non-finite route request")
		}
	}
	if r == nil || r.Type != pathfinding.PathfindNormal || len(r.Points) < 2 || len(r.Points) > 74 {
		return nil, fmt.Errorf("MMap route missing, incomplete or fallback")
	}
	if distance434(start, dest) < 0.25 {
		return nil, fmt.Errorf("already at destination; nontrivial route required")
	}
	if distance434(start, dest) > 10.5 {
		return nil, fmt.Errorf("destination exceeds short-probe limit")
	}
	if distance434(start, r.Points[0]) > 1 || distance434(dest, r.Points[len(r.Points)-1]) > 0.75 {
		return nil, fmt.Errorf("route endpoints do not match request")
	}
	if length := r.PathLength(); math.IsNaN(float64(length)) || length < 4 || length > 15 {
		return nil, fmt.Errorf("route length outside conservative limits")
	}
	var segments []Position434
	previous := start // authoritative anchor; never teleport to projected mesh start
	for i, p := range r.Points {
		if !finitePosition434(&Position434{X: p.X, Y: p.Y, Z: p.Z}) {
			return nil, fmt.Errorf("non-finite path point")
		}
		if i == 0 {
			continue
		}
		dx, dy, dz := float64(p.X-previous.X), float64(p.Y-previous.Y), float64(p.Z-previous.Z)
		horizontal := math.Hypot(dx, dy)
		if horizontal < 0.01 {
			if math.Abs(dz) > 0.05 {
				return nil, fmt.Errorf("vertical-only route segment")
			}
			continue
		}
		if math.Abs(dz)/horizontal > 0.3 {
			return nil, fmt.Errorf("route is too steep for initial ground probe")
		}
		steps := int(math.Ceil(math.Sqrt(dx*dx + dy*dy + dz*dz)))
		facing := math.Atan2(dy, dx)
		if facing < 0 {
			facing += 2 * math.Pi
		}
		for j := 1; j <= steps; j++ {
			fraction := float32(j) / float32(steps)
			segments = append(segments, Position434{X: previous.X + (p.X-previous.X)*fraction, Y: previous.Y + (p.Y-previous.Y)*fraction, Z: previous.Z + (p.Z-previous.Z)*fraction, Orientation: float32(facing)})
		}
		previous = p
	}
	if len(segments) < 2 || len(segments) > 24 {
		return nil, fmt.Errorf("invalid route segment count")
	}
	return segments, nil
}

type routeQuery434 struct {
	result *pathfinding.PathResult
	err    error
}
type navigationController434 struct {
	ctx                           context.Context
	result                        *NavigationAttempt434
	finder                        RouteFinder434
	query                         chan routeQuery434
	stage                         int
	readyAt, startedAt, stoppedAt uint32
	segmentStart                  Position434
}

func (c *navigationController434) tick(now uint32, send func(uint16, []byte) error) error {
	if err := c.ctx.Err(); err != nil {
		return err
	}
	a := c.result
	if c.stage == 0 {
		ready, err := a.World.playerReady()
		if err != nil {
			return err
		}
		if !ready {
			return nil
		}
		if c.readyAt == 0 {
			c.readyAt = now
			return nil
		}
		if now-c.readyAt < 1000 {
			return nil
		}
		p := a.World.Store.objects[a.World.Store.PlayerGUID]
		if p.Map != 1 || p.Movement == nil || p.Fields[FieldHealth434] == 0 {
			return fmt.Errorf("ground navigation requires healthy decoded map-1 player")
		}
		if _, err := tinyDestination434(*p.Movement); err != nil {
			return err
		}
		a.Initial = p.Movement.clone()
		a.Commanded = a.Initial.Position
		// Candidate only: it MUST pass through MMap search and strict validation.
		a.Requested = point434(a.Initial.Position)
		a.Requested.X += float32(7 * math.Cos(float64(a.Initial.Position.Orientation)))
		a.Requested.Y += float32(7 * math.Sin(float64(a.Initial.Position.Orientation)))
		start, dest, mapID := point434(a.Initial.Position), a.Requested, uint32(p.Map)
		c.query = make(chan routeQuery434, 1)
		go func() { r, err := c.finder.FindPath(mapID, start, dest); c.query <- routeQuery434{r, err} }()
		c.stage = 1
		return nil
	}
	if c.stage == 1 {
		select {
		case query := <-c.query:
			if query.err != nil {
				return fmt.Errorf("MMap query: %w", query.err)
			}
			segments, err := validateRoute434(point434(a.Initial.Position), a.Requested, query.result)
			if err != nil {
				return err
			}
			a.RawPath = pathfinding.PathResult{Type: query.result.Type, Points: append([]pathfinding.Point3D(nil), query.result.Points...)}
			a.Segments = segments
			if err := send(cataSetActiveMover, activeMover434(a.Initial.GUID)); err != nil {
				return err
			}
			a.Sent = append(a.Sent, cataSetActiveMover)
			c.stage = 2
		default:
			return nil
		}
	}
	if c.stage == 2 {
		c.segmentStart = a.Commanded
		c.segmentStart.Orientation = a.Segments[a.Executed].Orientation
		if err := c.send(send, cataMoveStartForward, c.segmentStart, moveForward434, now); err != nil {
			return err
		}
		c.startedAt = now
		c.stage = 3
		return nil
	}
	if c.stage == 3 || c.stage == 4 {
		end := a.Segments[a.Executed]
		duration := uint32(math.Ceil(1000 * distance434(point434(c.segmentStart), point434(end)) / float64(a.Initial.RunSpeed)))
		if duration < 400 {
			duration = 400
		}
		if c.stage == 3 && now-c.startedAt >= duration/2 {
			mid := end
			mid.X = (end.X + c.segmentStart.X) / 2
			mid.Y = (end.Y + c.segmentStart.Y) / 2
			mid.Z = (end.Z + c.segmentStart.Z) / 2
			if err := c.send(send, cataMoveHeartbeat, mid, moveForward434, now); err != nil {
				return err
			}
			c.stage = 4
			return nil
		}
		if c.stage == 4 && now-c.startedAt >= duration {
			if err := c.send(send, cataMoveStop, end, 0, now); err != nil {
				return err
			}
			a.Commanded = end
			a.Executed++
			if a.Executed == len(a.Segments) {
				c.stage = 5
				c.stoppedAt = now
			} else {
				c.stage = 2
			}
			return nil
		}
	}
	if c.stage == 5 && now-c.stoppedAt >= 1000 {
		c.stage = 6
	}
	return nil
}
func (c *navigationController434) send(send func(uint16, []byte) error, op uint16, p Position434, flags uint32, now uint32) error {
	m := c.result.Initial.clone()
	m.Position = p
	m.Flags = flags
	m.Timestamp = now
	m.HasTimestamp = true
	m.HasOrientation = true
	b, err := encodeMovement434(op, m)
	if err != nil {
		return err
	}
	if err := send(op, b); err != nil {
		return err
	}
	c.result.Sent = append(c.result.Sent, op)
	return nil
}

func NavigateCharacter434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instance string, finder RouteFinder434) (NavigationAttempt434, error) {
	a := NavigationAttempt434{World: WorldState434Result{Opcodes: map[uint16]int{}, WorldVariables: map[uint32]int32{}}}
	if name == "" || instance == "" || finder == nil {
		return a, fmt.Errorf("explicit character, instance and MMap finder required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := withWorld434(ctx, user, key, realm, func(w *worldWire434) error {
		roster, err := enumerateOnWorld434(w)
		if err != nil {
			return err
		}
		character, err := selectLoginCharacter434(roster, name)
		if err != nil {
			return err
		}
		a.World.Login.Character = character
		a.World.Store.PlayerGUID = character.GUID
		if err := w.send(cataPlayerLogin, loginGUID434(character.GUID)); err != nil {
			return err
		}
		controller := navigationController434{ctx: ctx, result: &a, finder: finder}
		return awaitSession434(ctx, w, strings.ToUpper(user), instance, &a.World.Login, openInstance434, a.World.observe, func() (bool, error) { return controller.stage == 6, nil }, controller.tick)
	})
	return a, err
}
func ValidateNavigationProof434(a NavigationAttempt434, final WorldState434Result) (Movement434, float64, error) {
	if a.Executed < 2 || a.Executed != len(a.Segments) || a.World.Store.PlayerGUID != final.Store.PlayerGUID || a.World.Login.Character.Name != final.Login.Character.Name || final.Login.Map != 1 {
		return Movement434{}, 0, fmt.Errorf("navigation proof identity/map/execution mismatch")
	}
	if ready, err := final.playerReady(); err != nil || !ready {
		return Movement434{}, 0, fmt.Errorf("fresh player proof missing: %v", err)
	}
	p := final.Store.objects[final.Store.PlayerGUID]
	if p.Movement == nil || p.Movement.Moving() {
		return Movement434{}, 0, fmt.Errorf("final stopped state missing")
	}
	m := p.Movement.clone()
	if distance434(point434(m.Position), point434(*p.Position)) > 0.001 {
		return m, 0, fmt.Errorf("fresh movement and object positions disagree")
	}
	errorToRequest := distance434(point434(m.Position), a.Requested)
	if errorToRequest > 0.75 || distance434(point434(m.Position), point434(a.Commanded)) > 0.05 {
		return m, errorToRequest, fmt.Errorf("fresh server position differs from destination/stop")
	}
	if math.Abs(math.Remainder(float64(m.Position.Orientation-a.Commanded.Orientation), 2*math.Pi)) > 0.01 {
		return m, errorToRequest, fmt.Errorf("fresh facing differs from final segment")
	}
	return m, errorToRequest, nil
}
