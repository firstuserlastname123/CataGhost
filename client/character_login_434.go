package client

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cataPlayerLogin          = 0x05b1
	cataConnectTo            = 0x0942
	cataContinuedSession     = 0x044d
	cataResumeComms          = 0x0140
	cataLoginVerifyWorld     = 0x2005
	cataCharacterLoginFailed = 0x4417
	cataTimeSyncRequest      = 0x3ca4
	cataTimeSyncResponse     = 0x3b0c
)

type CharacterLogin434Result struct {
	Character            Character434
	Map                  int32
	X, Y, Z, Orientation float32
	TimeSyncCounter      uint32
	InstanceAddress      string
}

// LoginCharacter434 selects an explicitly named roster character and proves
// world entry via LOGIN_VERIFY_WORLD plus the initial time-sync request/reply.
// expectedInstance pins the redirect target; no arbitrary server-supplied dial.
func LoginCharacter434(ctx context.Context, username string, key []byte, realm RealmInfo, name, expectedInstance string) (CharacterLogin434Result, error) {
	var result CharacterLogin434Result
	if name == "" || expectedInstance == "" {
		return result, fmt.Errorf("character name and expected instance address are required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err := withWorld434(ctx, username, key, realm, func(w *worldWire434) error {
		roster, err := enumerateOnWorld434(w)
		if err != nil {
			return err
		}
		character, err := selectLoginCharacter434(roster, name)
		if err != nil {
			return err
		}
		result.Character = character
		if err := w.send(cataPlayerLogin, loginGUID434(character.GUID)); err != nil {
			return fmt.Errorf("character login request: %w", err)
		}
		return awaitLogin434(ctx, w, strings.ToUpper(username), expectedInstance, &result)
	})
	return result, err
}

func selectLoginCharacter434(roster CharacterRoster434, name string) (Character434, error) {
	if len(roster.Characters) == 0 {
		return Character434{}, fmt.Errorf("QA account has no characters; create one separately before login validation")
	}
	var selected Character434
	count := 0
	for _, c := range roster.Characters {
		if c.Name == name {
			selected = c
			count++
		}
	}
	if count != 1 || selected.GUID == 0 {
		return Character434{}, fmt.Errorf("expected exactly one nonzero-GUID character named %q; found %d", name, count)
	}
	return selected, nil
}

func loginGUID434(guid uint64) []byte {
	var g [8]byte
	binary.LittleEndian.PutUint64(g[:], guid)
	b := []byte{0}
	for bit, i := range []int{2, 3, 0, 6, 4, 5, 1, 7} {
		if g[i] != 0 {
			b[0] |= 1 << uint(7-bit)
		}
	}
	for _, i := range []int{2, 7, 0, 3, 5, 6, 1, 4} {
		if g[i] != 0 {
			b = append(b, g[i]^1)
		}
	}
	return b
}

type loginPacket434 struct {
	op       uint16
	body     []byte
	err      error
	instance bool
}

func awaitLogin434(ctx context.Context, realm *worldWire434, username, expectedInstance string, result *CharacterLogin434Result) error {
	return awaitLoginWith434(ctx, realm, username, expectedInstance, result, openInstance434)
}

func awaitLoginWith434(ctx context.Context, realm *worldWire434, username, expectedInstance string, result *CharacterLogin434Result, connect func(context.Context, string, string, []byte, uint64) (*worldWire434, error)) error {
	ctx, cancel := context.WithCancel(ctx)
	var instance *worldWire434
	var readers sync.WaitGroup
	defer func() {
		cancel()
		realm.conn.Close()
		if instance != nil {
			instance.conn.Close()
		}
		readers.Wait()
	}()
	events := make(chan loginPacket434, 16)
	start := func(w *worldWire434, isInstance bool) {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				op, b, err := w.read()
				select {
				case events <- loginPacket434{op, b, err, isInstance}:
				case <-ctx.Done():
					return
				}
				if err != nil {
					return
				}
			}
		}()
	}
	start(realm, false)
	verified, resumed, synced := false, false, false
	started := time.Now()
	for packets := 0; packets < 512; packets++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("login deadline/cancellation: %w", ctx.Err())
		case p := <-events:
			if p.err != nil {
				return fmt.Errorf("login transport (instance=%t): %w", p.instance, p.err)
			}
			switch p.op {
			case cataCharacterLoginFailed:
				if len(p.body) != 1 {
					return fmt.Errorf("malformed character-login rejection")
				}
				return fmt.Errorf("character login rejected: code %d", p.body[0])
			case cataAuthResponse:
				return fmt.Errorf("unexpected authentication response during instance login")
			case cataConnectTo:
				if p.instance || instance != nil {
					return fmt.Errorf("unexpected repeated instance redirect")
				}
				link, address, err := redirect434(p.body)
				if err != nil {
					return err
				}
				if address != expectedInstance {
					return fmt.Errorf("instance redirect does not match expected address")
				}
				instance, err = connect(ctx, address, username, realm.key, link)
				if err != nil {
					return err
				}
				result.InstanceAddress = address
				start(instance, true)
			case cataResumeComms:
				if !p.instance || len(p.body) != 0 {
					return fmt.Errorf("invalid instance RESUME_COMMS")
				}
				resumed = true
			case cataLoginVerifyWorld:
				if !p.instance || !resumed {
					return fmt.Errorf("world verification before instance resume")
				}
				if err := verifyWorld434(p.body, result); err != nil {
					return err
				}
				verified = true
			case cataTimeSyncRequest:
				if !p.instance || len(p.body) != 4 {
					return fmt.Errorf("malformed time-sync request")
				}
				counter := binary.LittleEndian.Uint32(p.body)
				body := binary.LittleEndian.AppendUint32(nil, counter)
				body = binary.LittleEndian.AppendUint32(body, uint32(time.Since(started).Milliseconds()))
				if err := instance.send(cataTimeSyncResponse, body); err != nil {
					return err
				}
				result.TimeSyncCounter = counter
				synced = true
			}
			if verified && synced && resumed {
				return nil
			}
		}
	}
	return fmt.Errorf("login proof missing after 512 packets")
}

func verifyWorld434(b []byte, result *CharacterLogin434Result) error {
	if len(b) != 20 {
		return fmt.Errorf("invalid LOGIN_VERIFY_WORLD length %d", len(b))
	}
	result.Map = int32(binary.LittleEndian.Uint32(b))
	values := []*float32{&result.X, &result.Y, &result.Z, &result.Orientation}
	for i, v := range values {
		*v = math.Float32frombits(binary.LittleEndian.Uint32(b[4+i*4:]))
		if math.IsNaN(float64(*v)) || math.IsInf(float64(*v), 0) {
			return fmt.Errorf("invalid world position")
		}
	}
	return nil
}

func openInstance434(ctx context.Context, address, username string, key []byte, link uint64) (*worldWire434, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("instance connect: %w", err)
	}
	success := false
	defer func() {
		if !success {
			conn.Close()
		}
	}()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, err
	}
	challenge, err := challenge434(conn)
	if err != nil {
		return nil, err
	}
	body := continued434(username, key, link, challenge[32:36])
	header := binary.BigEndian.AppendUint16(nil, uint16(len(body)+4))
	header = binary.LittleEndian.AppendUint32(header, cataContinuedSession)
	if err := write434(conn, append(header, body...)); err != nil {
		return nil, err
	}
	w := &worldWire434{conn: conn, key: key, receive: packetCrypt434(key, challenge[:16]), transmit: packetCrypt434(key, challenge[16:32])}
	success = true
	return w, nil
}

func continued434(username string, key []byte, link uint64, challenge []byte) []byte {
	h := sha1.New()
	h.Write([]byte(username))
	h.Write(key)
	h.Write(challenge)
	d := h.Sum(nil)
	b := binary.LittleEndian.AppendUint64(nil, link)
	b = binary.LittleEndian.AppendUint64(b, 0)
	for _, i := range []int{5, 2, 6, 10, 8, 17, 11, 15, 7, 1, 4, 16, 0, 12, 14, 13, 18, 9, 19, 3} {
		b = append(b, d[i])
	}
	return b
}

func redirect434(b []byte) (uint64, string, error) {
	if len(b) != 269 || b[268] != 1 {
		return 0, "", fmt.Errorf("invalid instance redirect framing")
	}
	link := binary.LittleEndian.Uint64(b[:8])
	if (link>>32)&1 != 1 {
		return 0, "", fmt.Errorf("redirect is not an instance connection")
	}
	modBytes, _ := hex.DecodeString(connectModulus434)
	mod := bigIntFromBytes(modBytes)
	cipher := bigIntFromBytes(b[12:268])
	if cipher.Cmp(mod) >= 0 {
		return 0, "", fmt.Errorf("invalid redirect RSA value")
	}
	plain := bigIntToBytes(new(big.Int).Exp(cipher, big.NewInt(65537), mod))
	payload := make([]byte, 256)
	copy(payload, plain)
	address, err := redirectPayload434(payload)
	return link, address, err
}

func redirectPayload434(p []byte) (string, error) {
	if len(p) != 256 || p[255] != 0 {
		return "", fmt.Errorf("invalid redirect RSA payload")
	}
	pick := func(offsets []int) []byte {
		b := make([]byte, len(offsets))
		for i, pos := range offsets {
			b[i] = p[pos]
		}
		return b
	}
	address := pick(connectaddress434[:])
	kind := p[212]
	port := uint16(p[75]) | uint16(p[187])<<8
	if kind != 1 && kind != 2 || port == 0 {
		return "", fmt.Errorf("invalid redirect address type/port")
	}
	hmacKey, _ := hex.DecodeString(connectHMACKey434)
	h := hmac.New(sha1.New, hmacKey)
	h.Write(address)
	h.Write(binary.LittleEndian.AppendUint32(nil, uint32(kind)))
	h.Write(binary.LittleEndian.AppendUint16(nil, port))
	h.Write(pick(connectHaiku434[:]))
	h.Write(pick(connectPiDigits434[:]))
	h.Write(p[83:84])
	if !hmac.Equal(h.Sum(nil), pick(connecthmac434[:])) {
		return "", fmt.Errorf("redirect HMAC verification failed")
	}
	if kind == 1 {
		address = address[:4]
	}
	return net.JoinHostPort(net.IP(address).String(), strconv.Itoa(int(port))), nil
}
