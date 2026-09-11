package client

import (
	"context"
	"crypto/rc4"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestMovement434EncryptedSession(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		t.Run(fmt.Sprint(compressed), func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			left, right := net.Pipe()
			defer right.Close()
			fixture := redirectFixture434(t)
			go func() { write434(right, frame434(cataConnectTo, fixture, nil)) }()
			done := make(chan error, 1)
			go func() {
				done <- mockInstanceTraffic434(ln, compressed, func(conn net.Conn, recv *rc4.Cipher, send func(uint16, []byte) error) error {
					for _, expected := range []uint32{0x3314, 0x7814, 0x3914, 0x320a} {
						op, b, err := readClient434(conn, recv)
						if err != nil || op != expected {
							return fmt.Errorf("unexpected outbound %x want %x: %v", op, expected, err)
						}
						if op == 0x3314 {
							if len(b) != 2 || b[0] != 0x10 || b[1] != 0x10 {
								return fmt.Errorf("wrong enumerated active mover")
							}
							continue
						}
						m, err := decodeMovement434(uint16(op), b)
						if err != nil || m.GUID != 17 {
							return fmt.Errorf("bad movement: %v", err)
						}
						if op == 0x320a {
							if m.Flags != 0 || m.Timestamp == 0 {
								return fmt.Errorf("not stopped")
							}
							// Independent server movement update, retaining persistent zlib and
							// encrypted header history from initial login/object packets.
							body, err := encodeMovement434(0x79a2, m)
							if err != nil {
								return err
							}
							if err := send(0x79a2, body); err != nil {
								return err
							}
							if err := send(0x3ca4, []byte{8, 0, 0, 0}); err != nil {
								return err
							}
							op, b, err = readClient434(conn, recv)
							if err != nil || op != 0x3b0c || len(b) != 8 || binary.LittleEndian.Uint32(b) != 8 {
								return fmt.Errorf("time-sync regression after movement: %x %v", op, err)
							}
						}
					}
					return nil
				}, updateFixture434(livingFixture434(17, 4, false)))
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			s := MovementAttempt434{World: WorldState434Result{Store: ObjectStore434{PlayerGUID: 17}, Opcodes: map[uint16]int{}, WorldVariables: map[uint32]int32{}}}
			s.World.Login.Character = Character434{GUID: 17, Name: "Synthetic"}
			controller := tinyMovement434{world: &s.World, result: &s}
			err = awaitSession434(ctx, &worldWire434{conn: left, key: key434()}, "ACCOUNT", "127.0.0.1:8087", &s.World.Login, func(ctx context.Context, address, user string, key []byte, link uint64) (*worldWire434, error) {
				return openInstance434(ctx, ln.Addr().String(), user, key, link)
			}, s.World.observe, func() (bool, error) { return controller.stage == 4, nil }, controller.tick)
			if err != nil {
				t.Fatal(err)
			}
			if s.World.Opcodes[0x79a2] != 1 || s.World.Login.TimeSyncCounter != 8 {
				t.Fatal("movement/time-sync not processed")
			}
			if s.World.Store.objects[17].Position.X != s.Stop.Position.X {
				t.Fatal("server update not applied")
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("unclean shutdown")
			}
		})
	}
}

func TestMovement434CanceledSessionSendsNothing(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := awaitSession434(ctx, &worldWire434{conn: left}, "SYNTHETIC", "localhost:1", &CharacterLogin434Result{}, openInstance434, nil, nil, func(uint32, func(uint16, []byte) error) error { called = true; return nil })
	if err == nil || called {
		t.Fatal("canceled session started movement")
	}
}
