package client

import (
	"context"
	"crypto/rc4"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

// Actual encrypted continued-session sockets exercise the production tick and
// cleanup path. Synthetic combat only; no QA server is contacted by this test.
func TestObjective434TransportCancellationAndLoss(t *testing.T) {
	for _, kind := range []string{"pet", "combat"} {
		for _, loss := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/loss=%v", kind, loss), func(t *testing.T) {
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer ln.Close()
				left, right := net.Pipe()
				defer right.Close()
				fixture := redirectFixture434(t)
				go func() { _ = write434(right, frame434(cataConnectTo, fixture, nil)) }()
				root, cancelRoot := context.WithTimeout(context.Background(), 4*time.Second)
				defer cancelRoot()
				user, cancelUser := context.WithCancel(root)
				defer cancelUser()
				combat, _, target := combatFixture434(t)
				pet, _, _ := petFixture434(t)
				pet.readOnly = true
				world := &combat.result.Observation.World
				observe := combat.observe
				tick := func(now uint32, send func(uint16, []byte) error) error { return combat.tickContext(user, now, send) }
				sessionCtx := root
				if kind == "pet" {
					world = &pet.result.Observation.World
					observe = pet.observe
					tick = pet.tick
					sessionCtx = user
				}
				peerLost := errors.New("synthetic peer disconnect")
				done := make(chan error, 1)
				go func() {
					done <- mockInstanceTraffic434(ln, true, func(conn net.Conn, recv *rc4.Cipher, send func(uint16, []byte) error) error {
						expected := []uint32{0x4924}
						if kind == "combat" {
							expected = []uint32{uint32(cataSetActiveMover), uint32(cataMoveHeartbeat), cataSetSelection, cataAttackSwing434}
						}
						for _, want := range expected {
							op, b, err := readClient434(conn, recv)
							if err != nil || op != want {
								return fmt.Errorf("outbound %04X want %04X: %v", op, want, err)
							}
							if op == cataAttackSwing434 && (len(b) != 8 || binary.LittleEndian.Uint64(b) != target.GUID) {
								return fmt.Errorf("wrong attack GUID")
							}
						}
						if loss {
							_ = conn.Close()
							return peerLost
						}
						cancelUser()
						if kind == "combat" {
							for _, want := range []uint32{cataAttackStop434, cataSetSelection} {
								op, b, err := readClient434(conn, recv)
								if err != nil || op != want {
									return fmt.Errorf("cancel stop %04X want %04X: %v", op, want, err)
								}
								if op == cataAttackStop434 && len(b) != 0 {
									return fmt.Errorf("nonempty stop")
								}
								if op == cataSetSelection && (len(b) != 8 || binary.LittleEndian.Uint64(b) != 0) {
									return fmt.Errorf("target not cleared")
								}
							}
						}
						return nil
					})
				}()
				err = awaitSession434(sessionCtx, &worldWire434{conn: left, key: key434()}, "ACCOUNT", "127.0.0.1:8087", &world.Login, func(ctx context.Context, address, user string, key []byte, link uint64) (*worldWire434, error) {
					return openInstance434(ctx, ln.Addr().String(), user, key, link)
				}, observe, func() (bool, error) { return false, nil }, tick)
				if err == nil {
					t.Fatal("interrupted session reported success")
				}
				if !loss && !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
				if combat.result.Passed || pet.result.SafetyProven {
					t.Fatal("false success after interruption")
				}
				if kind == "combat" && !loss && !combat.result.Stopped {
					t.Fatal("no final stop on user cancellation")
				}
				if kind == "pet" && (pet.result.ControlSent || len(pet.result.Sent) != 1) {
					t.Fatal("uncertain pet sent control")
				}
				select {
				case peerErr := <-done:
					if loss {
						if !errors.Is(peerErr, peerLost) {
							t.Fatal(peerErr)
						}
					} else if peerErr != nil {
						t.Fatal(peerErr)
					}
				case <-time.After(time.Second):
					t.Fatal("session reader cleanup blocked")
				}
				_ = right.SetReadDeadline(time.Now().Add(time.Second))
				one := make([]byte, 1)
				if _, e := right.Read(one); e != io.EOF {
					t.Fatalf("realm not closed: %v", e)
				}
			})
		}
	}
}
