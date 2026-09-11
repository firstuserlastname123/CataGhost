package client

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

func (s *ObjectStore434) ApplyMovement(body []byte) error {
	m, err := decodeMovement434(cataMoveUpdate, body)
	if err != nil {
		return err
	}
	o := s.objects[m.GUID]
	if o == nil || !o.Created {
		return nil
	} // no invented lifetime/map
	s.revision++
	o.Revision = s.revision
	o.Position = &m.Position
	o.Movement = &m
	return nil
}

type MovementAttempt434 struct {
	World   WorldState434Result
	Initial Movement434
	Stop    Movement434 // proposal; not server proof
	Sent    []uint16
}

type tinyMovement434 struct {
	world                         *WorldState434Result
	result                        *MovementAttempt434
	stage                         int
	readyAt, startedAt, stoppedAt uint32
}

func tinyDestination434(m Movement434) (Movement434, error) {
	if math.IsNaN(float64(m.RunSpeed)) || math.IsInf(float64(m.RunSpeed), 0) {
		return m, fmt.Errorf("invalid run speed")
	}
	if m.GUID == 0 || !finitePosition434(&m.Position) || m.Flags != 0 || m.Flags2 != 0 || m.Transport != nil || m.Fall != nil || m.Spline || m.RunSpeed <= 0 || m.RunSpeed > 15 {
		return m, fmt.Errorf("tiny movement requires a stationary ordinary ground player with known run speed")
	}
	n := m.clone()
	n.Position.X += float32(math.Cos(float64(m.Position.Orientation)))
	n.Position.Y += float32(math.Sin(float64(m.Position.Orientation)))
	if !finitePosition434(&n.Position) {
		return m, fmt.Errorf("invalid movement destination")
	}
	return n, nil
}
func (t *tinyMovement434) tick(now uint32, send func(uint16, []byte) error) error {
	if t.stage == 0 {
		ready, err := t.world.playerReady()
		if err != nil {
			return err
		}
		if !ready {
			return nil
		}
		if t.readyAt == 0 {
			t.readyAt = now
			return nil
		}
		if now-t.readyAt < 1000 {
			return nil
		}
		p := t.world.Store.objects[t.world.Store.PlayerGUID]
		if p.Map != 1 || p.Movement == nil {
			return fmt.Errorf("tiny QA movement requires map 1 and decoded player movement")
		}
		if health, ok := p.Fields[FieldHealth434]; !ok || health == 0 {
			return fmt.Errorf("player health unavailable or zero")
		}
		t.result.Initial = p.Movement.clone()
		target, err := tinyDestination434(t.result.Initial)
		if err != nil {
			return err
		}
		t.result.Stop = target
		if err := send(cataSetActiveMover, activeMover434(p.GUID)); err != nil {
			return err
		}
		t.result.Sent = append(t.result.Sent, cataSetActiveMover)
		m := t.result.Initial
		m.Flags = moveForward434
		m.Timestamp = now
		m.HasTimestamp = true
		m.HasOrientation = true
		if err := t.send(send, cataMoveStartForward, m); err != nil {
			return err
		}
		t.startedAt = now
		t.stage = 1
		return nil
	}
	// At most 1 unit, at a rate below the decoded server run speed.
	duration := uint32(math.Ceil(1000 / float64(t.result.Initial.RunSpeed)))
	if duration < 400 {
		duration = 400
	}
	if t.stage == 1 && now-t.startedAt >= duration/2 {
		m := t.result.Initial
		m.Position.X = (m.Position.X + t.result.Stop.Position.X) / 2
		m.Position.Y = (m.Position.Y + t.result.Stop.Position.Y) / 2
		m.Flags = moveForward434
		m.Timestamp = now
		m.HasTimestamp = true
		m.HasOrientation = true
		if err := t.send(send, cataMoveHeartbeat, m); err != nil {
			return err
		}
		t.stage = 2
		return nil
	}
	if t.stage == 2 && now-t.startedAt >= duration {
		m := t.result.Stop
		m.Flags = 0
		m.Timestamp = now
		m.HasTimestamp = true
		m.HasOrientation = true
		if err := t.send(send, cataMoveStop, m); err != nil {
			return err
		}
		t.result.Stop = m
		t.stoppedAt = now
		t.stage = 3
	}
	if t.stage == 3 && now-t.stoppedAt >= 1000 {
		t.stage = 4
	}
	return nil
}
func (t *tinyMovement434) send(send func(uint16, []byte) error, op uint16, m Movement434) error {
	b, err := encodeMovement434(op, m)
	if err != nil {
		return err
	}
	if err := send(op, b); err != nil {
		return err
	}
	t.result.Sent = append(t.result.Sent, op)
	return nil
}

// MoveCharacter434 sends one bounded ground movement and closes. Its result is
// deliberately an attempt: ValidateMovementProof434 needs a fresh server snapshot.
func MoveCharacter434(ctx context.Context, user string, key []byte, realm RealmInfo, name, instanceAddress string) (MovementAttempt434, error) {
	s := MovementAttempt434{World: WorldState434Result{Opcodes: make(map[uint16]int), WorldVariables: make(map[uint32]int32)}}
	if name == "" || instanceAddress == "" {
		return s, fmt.Errorf("explicit character and instance address required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := withWorld434(ctx, user, key, realm, func(w *worldWire434) error {
		roster, err := enumerateOnWorld434(w)
		if err != nil {
			return err
		}
		c, err := selectLoginCharacter434(roster, name)
		if err != nil {
			return err
		}
		s.World.Login.Character = c
		s.World.Store.PlayerGUID = c.GUID
		if err := w.send(cataPlayerLogin, loginGUID434(c.GUID)); err != nil {
			return err
		}
		t := tinyMovement434{world: &s.World, result: &s}
		return awaitSession434(ctx, w, strings.ToUpper(user), instanceAddress, &s.World.Login, openInstance434, s.World.observe, func() (bool, error) { return t.stage == 4, nil }, t.tick)
	})
	return s, err
}

// The stop packet alone carries the final coordinate. A fresh LOGIN_VERIFY_WORLD
// and player create matching it prove that TCPP accepted that stop/position.
func ValidateMovementProof434(attempt MovementAttempt434, final WorldState434Result) (Movement434, error) {
	if attempt.World.Store.PlayerGUID != final.Store.PlayerGUID || attempt.World.Login.Character.Name != final.Login.Character.Name || final.Login.Map != 1 || attempt.World.Login.Map != final.Login.Map {
		return Movement434{}, fmt.Errorf("movement verification identity/map mismatch")
	}
	if ready, err := final.playerReady(); err != nil || !ready {
		return Movement434{}, fmt.Errorf("final player proof missing: %v", err)
	}
	p := final.Store.objects[final.Store.PlayerGUID]
	if p.Movement == nil || p.Movement.Moving() {
		return Movement434{}, fmt.Errorf("final movement state missing or moving")
	}
	f := p.Movement.clone()
	dx, dy, dz := float64(f.Position.X-attempt.Initial.Position.X), float64(f.Position.Y-attempt.Initial.Position.Y), float64(f.Position.Z-attempt.Initial.Position.Z)
	distance := math.Hypot(dx, dy)
	if distance < 0.9 || distance > 1.1 || math.Abs(dz) > 0.05 {
		return f, fmt.Errorf("server displacement is not the intended one-unit ground movement")
	}
	for _, d := range []float32{f.Position.X - attempt.Stop.Position.X, f.Position.Y - attempt.Stop.Position.Y, f.Position.Z - attempt.Stop.Position.Z} {
		if math.Abs(float64(d)) > 0.02 {
			return f, fmt.Errorf("server position differs from stop position")
		}
	}
	if math.Abs(math.Remainder(float64(f.Position.Orientation-attempt.Initial.Position.Orientation), 2*math.Pi)) > 0.01 {
		return f, fmt.Errorf("server facing changed")
	}
	return f, nil
}
