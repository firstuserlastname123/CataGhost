package client

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/azerothcore/AzerothGhost/pathfinding"
)

type routeFunc434 func(uint32, pathfinding.Point3D, pathfinding.Point3D) (*pathfinding.PathResult, error)

func (f routeFunc434) FindPath(m uint32, s, d pathfinding.Point3D) (*pathfinding.PathResult, error) {
	return f(m, s, d)
}
func navFixture434(t *testing.T) *NavigationAttempt434 {
	t.Helper()
	a := &NavigationAttempt434{World: WorldState434Result{Store: ObjectStore434{PlayerGUID: 17}, Opcodes: map[uint16]int{}, WorldVariables: map[uint32]int32{}}}
	if err := a.World.Store.ApplyUpdate(updateFixture434(livingFixture434(17, 4, false))); err != nil {
		t.Fatal(err)
	}
	a.World.Login = CharacterLogin434Result{Character: Character434{GUID: 17, Name: "Synthetic"}, Map: 1, X: 1.25, Y: -2.5, Z: 3.75, Orientation: 0.5}
	return a
}
func navPath434(s, d pathfinding.Point3D) *pathfinding.PathResult {
	return &pathfinding.PathResult{Type: pathfinding.PathfindNormal, Points: []pathfinding.Point3D{s, {X: (s.X + d.X) / 2, Y: (s.Y + d.Y) / 2, Z: (s.Z + d.Z) / 2}, d}}
}
func TestNavigation434RouteValidation(t *testing.T) {
	s := pathfinding.Point3D{}
	d := pathfinding.Point3D{X: 7}
	r := navPath434(s, d)
	segments, err := validateRoute434(s, d, r)
	if err != nil {
		t.Fatal(err)
	}
	previous := Position434{}
	for _, p := range segments {
		if distance434(point434(previous), point434(p)) > 1.001 || p.Orientation != 0 {
			t.Fatal("segment bound/facing", p)
		}
		previous = p
	}
	if previous.X != 7 || len(segments) < 7 {
		t.Fatal(segments)
	}
	for _, bad := range []*pathfinding.PathResult{nil, {}, {Type: pathfinding.PathfindNopath, Points: r.Points}, {Type: pathfinding.PathfindNormal | pathfinding.PathfindNotUsingPath, Points: r.Points}, {Type: pathfinding.PathfindIncomplete, Points: r.Points}, {Type: pathfinding.PathfindNormal, Points: []pathfinding.Point3D{d, s}}, {Type: pathfinding.PathfindNormal, Points: []pathfinding.Point3D{s, {X: 3, Z: 8}, d}}, {Type: pathfinding.PathfindNormal, Points: []pathfinding.Point3D{s, {X: float32(math.NaN())}, d}}} {
		if _, err := validateRoute434(s, d, bad); err == nil {
			t.Fatal("bad route accepted", bad)
		}
	}
	if _, err := validateRoute434(s, s, r); err == nil {
		t.Fatal("trivial route accepted")
	}
}
func TestNavigation434ExecutorAndProof(t *testing.T) {
	a := navFixture434(t)
	original := a.World.Store.Objects()
	var requestedMap uint32
	var requestedStart, requestedEnd pathfinding.Point3D
	finder := routeFunc434(func(m uint32, s, d pathfinding.Point3D) (*pathfinding.PathResult, error) {
		requestedMap = m
		requestedStart = s
		requestedEnd = d
		return navPath434(s, d), nil
	})
	c := navigationController434{ctx: context.Background(), result: a, finder: finder}
	var ops []uint16
	var last Position434
	var haveLast bool
	send := func(op uint16, b []byte) error {
		ops = append(ops, op)
		if op == cataSetActiveMover {
			return nil
		}
		if op != cataMoveStartForward && op != cataMoveHeartbeat && op != cataMoveStop {
			return errors.New("unrelated gameplay")
		}
		m, err := decodeMovement434(op, b)
		if err != nil {
			return err
		}
		if m.GUID != 17 {
			return errors.New("identity lost")
		}
		if haveLast && distance434(point434(last), point434(m.Position)) > 1.001 {
			return errors.New("teleport-size jump")
		}
		last = m.Position
		haveLast = true
		if op == cataMoveStop && m.Moving() {
			return errors.New("stop remains moving")
		}
		return nil
	}
	for _, now := range []uint32{100, 1100} {
		if err := c.tick(now, send); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case q := <-c.query:
		c.query <- q
	case <-time.After(time.Second):
		t.Fatal("query stuck")
	}
	for now := uint32(1200); now < 10000 && c.stage != 6; now += 100 {
		if err := c.tick(now, send); err != nil {
			t.Fatal(err)
		}
	}
	if requestedMap != 1 || requestedStart != (pathfinding.Point3D{X: 1.25, Y: -2.5, Z: 3.75}) || requestedEnd != a.Requested {
		t.Fatal("incorrect path request")
	}
	if c.stage != 6 || a.Executed != len(a.Segments) || a.Executed < 2 || len(ops) != 1+3*a.Executed {
		t.Fatal("incomplete multi-segment sequence", ops)
	}
	for i := 1; i < len(ops); i += 3 {
		if !reflect.DeepEqual(ops[i:i+3], []uint16{cataMoveStartForward, cataMoveHeartbeat, cataMoveStop}) {
			t.Fatal(ops)
		}
	}
	if !reflect.DeepEqual(original, a.World.Store.Objects()) {
		t.Fatal("commands mutated authoritative store")
	}
	final := navFixture434(t).World
	pose := a.Commanded
	final.Login.X = pose.X
	final.Login.Y = pose.Y
	final.Login.Z = pose.Z
	final.Login.Orientation = pose.Orientation
	obj := final.Store.objects[17]
	obj.Position = &pose
	m := obj.Movement.clone()
	m.Position = pose
	obj.Movement = &m
	if _, errorToDest, err := ValidateNavigationProof434(*a, final); err != nil || errorToDest > 0.001 {
		t.Fatal(errorToDest, err)
	}
	final.Store.objects[17].Position.X += 1
	final.Login.X += 1
	if _, _, err := ValidateNavigationProof434(*a, final); err == nil {
		t.Fatal("wrong server destination passed")
	}
}
func TestNavigation434Failures(t *testing.T) {
	for _, name := range []string{"query", "no-path", "send", "cancel", "timeout"} {
		t.Run(name, func(t *testing.T) {
			a := navFixture434(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if name == "timeout" {
				var stop context.CancelFunc
				ctx, stop = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer stop()
			}
			if name == "cancel" {
				cancel()
			}
			finder := routeFunc434(func(_ uint32, s, d pathfinding.Point3D) (*pathfinding.PathResult, error) {
				if name == "query" {
					return nil, errors.New("query failure")
				}
				if name == "no-path" {
					return &pathfinding.PathResult{Type: pathfinding.PathfindNopath}, nil
				}
				return navPath434(s, d), nil
			})
			c := navigationController434{ctx: ctx, result: a, finder: finder}
			sends := 0
			send := func(uint16, []byte) error { sends++; return errors.New("send failure") }
			err := c.tick(100, send)
			if err == nil {
				err = c.tick(1100, send)
				if err == nil {
					select {
					case q := <-c.query:
						c.query <- q
					case <-time.After(time.Second):
						t.Fatal("query stuck")
					}
					err = c.tick(1200, send)
				}
			}
			if err == nil || a.Executed != 0 {
				t.Fatal("failure not propagated")
			}
			if name != "send" && sends != 0 {
				t.Fatal("movement sent on failed preflight")
			}
		})
	}
}
