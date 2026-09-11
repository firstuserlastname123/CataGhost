package client

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// Build-15595 realm authentication only. Layouts follow TCPP's
// AuthenticationPackets.cpp, WorldSocket.cpp and WorldPacketCrypt.cpp.
const (
	cataAuthChallenge = 0x4542
	cataAuthSession   = 0x0449
	cataAuthResponse  = 0x5DB6
	cataServerBanner  = "WORLD OF WARCRAFT CONNECTION - SERVER TO CLIENT"
	cataClientBanner  = "WORLD OF WARCRAFT CONNECTION - CLIENT TO SERVER"
)

// AuthenticateWorld434 authenticates the selected realm and closes the socket.
// It never enumerates characters or enters the gameplay packet dispatcher.
// Queued sessions are reported as errors; queue progression is not implemented.
func AuthenticateWorld434(ctx context.Context, username string, key []byte, realm RealmInfo) error {
	return withWorld434(ctx, username, key, realm, nil)
}

func withWorld434(ctx context.Context, username string, key []byte, realm RealmInfo, after func(*worldWire434) error) error {
	username = strings.ToUpper(username)
	if len(username) == 0 || len(username) > 255 || strings.IndexByte(username, 0) >= 0 {
		return fmt.Errorf("world auth: account length must be 1..255 bytes without NUL")
	}
	if len(key) != 40 {
		return fmt.Errorf("world auth: session key must be 40 bytes")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", realm.Address)
	if err != nil {
		return fmt.Errorf("world connect: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	return handshake434(conn, username, key, realm.ID, after)
}

func worldAuth434(conn net.Conn, username string, key []byte, realmID uint32) error {
	return handshake434(conn, username, key, realmID, nil)
}

func handshake434(conn net.Conn, username string, key []byte, realmID uint32, after func(*worldWire434) error) error {
	challenge, err := challenge434(conn)
	if err != nil {
		return err
	}
	var seed [4]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return fmt.Errorf("client challenge: %w", err)
	}
	body := session434(username, key, realmID, seed[:], challenge[32:36])
	packet := make([]byte, 6, 6+len(body))
	binary.BigEndian.PutUint16(packet[:2], uint16(len(body)+4))
	binary.LittleEndian.PutUint32(packet[2:], cataAuthSession)
	if err := write434(conn, append(packet, body...)); err != nil {
		return fmt.Errorf("auth session: %w", err)
	}
	wire := &worldWire434{conn: conn, receive: receiveCrypt434(key), key: key}
	// TCPP may send unsolicited pre-auth traffic (e.g. Warden). Consume framed
	// packets without invoking gameplay handlers, under the connection deadline.
	for packets := 0; packets < 64; packets++ {
		op, data, err := wire.read()
		if err != nil {
			return fmt.Errorf("auth response framing/decryption: %w", err)
		}
		if op == cataAuthResponse {
			if err := response434(data); err != nil {
				return err
			}
			if after != nil {
				return after(wire)
			}
			return nil
		}
	}
	return fmt.Errorf("auth response missing after 64 packets")
}

func session434(account string, key []byte, realm uint32, local, server []byte) []byte {
	h := sha1.New()
	h.Write([]byte(account))
	h.Write([]byte{0, 0, 0, 0})
	h.Write(local)
	h.Write(server)
	h.Write(key)
	d := h.Sum(nil)
	b := make([]byte, 9) // LoginServerID, BattlegroupID, LoginServerType = 0
	b = append(b, d[10], d[18], d[12], d[5])
	b = binary.LittleEndian.AppendUint64(b, 3) // Clientless value; TCPP does not validate DoS response.
	b = append(b, d[15], d[9], d[19], d[4], d[7], d[16], d[3])
	b = binary.LittleEndian.AppendUint16(b, 15595)
	b = append(b, d[8])
	b = binary.LittleEndian.AppendUint32(b, realm)
	b = append(b, 1, d[17], d[6], d[0], d[1], d[11]) // BuildType = 1
	b = append(b, local...)
	b = append(b, d[2])
	b = binary.LittleEndian.AppendUint32(b, 0) // RegionID
	b = append(b, d[14], d[13])
	b = binary.LittleEndian.AppendUint32(b, 0) // Empty addon buffer accepted by TCPP.
	// MSB-first: IPv6=0, 12-bit length, then three zero padding bits.
	b = append(b, byte(len(account)>>5), byte(len(account)<<3))
	return append(b, account...)
}

func receiveCrypt434(key []byte) *rc4.Cipher {
	seed := []byte{0xCC, 0x98, 0xAE, 0x04, 0xE8, 0x97, 0xEA, 0xCA, 0x12, 0xDD, 0xC0, 0x93, 0x42, 0x91, 0x53, 0x57}
	h := hmac.New(sha1.New, seed)
	h.Write(key)
	c, _ := rc4.NewCipher(h.Sum(nil)) // SHA1 always yields a valid 20-byte RC4 key.
	var drop [1024]byte
	c.XORKeyStream(drop[:], drop[:])
	return c
}

func write434(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}

func read434(r io.Reader, crypt *rc4.Cipher) (uint16, []byte, error) {
	var h [5]byte
	if _, err := io.ReadFull(r, h[:4]); err != nil {
		return 0, nil, err
	}
	if crypt != nil {
		crypt.XORKeyStream(h[:4], h[:4])
	}
	size := uint32(binary.BigEndian.Uint16(h[:2]))
	op := binary.LittleEndian.Uint16(h[2:4])
	if h[0]&0x80 != 0 {
		if _, err := io.ReadFull(r, h[4:]); err != nil {
			return 0, nil, err
		}
		if crypt != nil {
			crypt.XORKeyStream(h[4:], h[4:])
		}
		size = uint32(h[0]&0x7f)<<16 | uint32(h[1])<<8 | uint32(h[2])
		op = binary.LittleEndian.Uint16(h[3:])
	}
	if size < 2 || size > 1024*1024 {
		return 0, nil, fmt.Errorf("invalid server packet size %d", size)
	}
	b := make([]byte, size-2)
	_, err := io.ReadFull(r, b)
	return op, b, err
}

func response434(b []byte) error {
	if len(b) < 2 {
		return fmt.Errorf("truncated auth response")
	}
	wait := b[0]&0x80 != 0
	successBit := byte(0x40)
	if wait {
		successBit = 0x20
	}
	success := b[0]&successBit != 0
	resultAt := 1
	if success {
		resultAt += 15
	}
	expected := resultAt + 1
	if wait {
		expected += 4
	}
	if len(b) != expected {
		return fmt.Errorf("invalid auth response length: got %d, want %d", len(b), expected)
	}
	if b[resultAt] != 12 {
		return fmt.Errorf("world authentication rejected: result %d", b[resultAt])
	}
	if wait {
		return fmt.Errorf("world authentication queued: position %d", binary.LittleEndian.Uint32(b[resultAt+1:]))
	}
	if !success {
		return fmt.Errorf("auth success missing account information")
	}
	return nil
}

func challenge434(conn net.Conn) ([]byte, error) {
	var size [2]byte
	if _, err := io.ReadFull(conn, size[:]); err != nil {
		return nil, fmt.Errorf("server banner: %w", err)
	}
	if binary.BigEndian.Uint16(size[:]) != uint16(len(cataServerBanner)) {
		return nil, fmt.Errorf("invalid server banner length")
	}
	banner := make([]byte, len(cataServerBanner))
	if _, err := io.ReadFull(conn, banner); err != nil {
		return nil, fmt.Errorf("server banner: %w", err)
	}
	if string(banner) != cataServerBanner {
		return nil, fmt.Errorf("invalid server banner")
	}
	binary.BigEndian.PutUint16(size[:], uint16(len(cataClientBanner)))
	if err := write434(conn, append(size[:], cataClientBanner...)); err != nil {
		return nil, fmt.Errorf("client banner: %w", err)
	}
	op, challenge, err := read434(conn, nil)
	if err != nil {
		return nil, fmt.Errorf("auth challenge: %w", err)
	}
	if op != cataAuthChallenge || len(challenge) != 37 {
		return nil, fmt.Errorf("invalid auth challenge: opcode 0x%04X, length %d", op, len(challenge))
	}
	return challenge, nil
}
