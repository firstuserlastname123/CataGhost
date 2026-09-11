# Cataclysm 4.3.4 build 15595: character login only

Authority: local TCPP aa47817dfb. CharacterPackets.cpp PlayerLogin::Read,
CharacterHandler.cpp HandlePlayerLoginOpcode/HandleContinuePlayerLogin/
HandlePlayerLogin, AuthenticationPackets.cpp ConnectTo::Write and
AuthContinuedSession::Read, WorldSocket.cpp HandleAuthContinuedSessionCallback,
World.cpp ProcessLinkInstanceSocket, Player.cpp SendInitialPacketsAfterAddToMap,
MovementHandler.cpp HandleTimeSyncResp, WorldSession.cpp SendTimeSync.
Secondary reference: Clientless 85422ae CharacterHandler.cpp and MiscHandler.cpp.
WowPacketParser's build-15595 reference fetch was unavailable in this session.

## Sequence and proof

Reuse the same authenticated realm socket for enumeration and login. Require an
explicit exact character name, resolve it against that roster, and send its GUID.
An empty roster, absent name, duplicate name or zero GUID fails before login.
No character is created. Preserve independent receive and transmit RC4 streams.

CMSG_PLAYER_LOGIN is 0x05B1; payload is one MSB-first presence-mask byte in GUID
index order 2,3,0,6,4,5,1,7, followed by nonzero GUID bytes XOR 1 in order
2,7,0,3,5,6,1,4. GUID 2 therefore produces 20 03. The six-byte client header
is BE16(payload length + 4), LE32(opcode); only the header is encrypted.

QA has LegacyConnectionModeEnabled=0. The server sends SMSG_CONNECT_TO (0x0942)
on the realm connection: LE64 link key, LE32 serial, 256 little-endian RSA bytes,
u8 connection type (1 = instance). Recover the payload with the protocol's public
RSA modulus and exponent 65537. Invert TCPP's 255-byte field permutation, verify
its HMAC-SHA1, and require the address to equal the explicitly supplied expected
instance endpoint. Do not follow arbitrary redirect addresses. Public constants
and byte offsets are in instance_redirect_434.go. The synthetic redirect fixture
was constructed independently from the TCPP writer and RSA-verified with .NET;
it contains a fabricated link, not a live session. No private RSA key is stored.

Keep the realm socket open. Connect the instance socket and perform the same
length-prefixed connectivity initializer and 37-byte challenge exchange.
Send plaintext CMSG_AUTH_CONTINUED_SESSION (0x044D), using the normal six-byte
client header. Body: LE64 link key, LE64 DoS response (0; ignored by TCPP), then
SHA1(uppercase account || 40-byte session key || four-byte instance challenge)
bytes in order 5,2,6,10,8,17,11,15,7,1,4,16,0,12,14,13,18,9,19,3.
Initialize separate instance RC4-drop1024 streams: HMAC-SHA1 with challenge
bytes 0..15 for receive and 16..31 for transmit. These are different from the
fixed realm seeds. Instance authentication links to the existing realm session.

Require empty SMSG_RESUME_COMMS (0x0140) on the instance connection. Then require
SMSG_LOGIN_VERIFY_WORLD (0x2005): LE32 map and four LE float32 values X,Y,Z,O.
TCPP emits verification after loading the selected character and its add-to-map
path. Reject malformed lengths and nonfinite positions.

Also require SMSG_TIME_SYNC_REQ (0x3CA4, LE32 counter), which is emitted by
SendInitialPacketsAfterAddToMap. Send CMSG_TIME_SYNC_RESP (0x3B0C) on the instance
socket: LE32 counter then LE32 monotonic elapsed milliseconds. The server sends
no acknowledgment of this reply; successful write is not claimed as proof of
server processing. Verification plus initial sync request is the entry proof.
Close both sockets once the required reply is written. No movement begins.

## Other packets, errors and boundaries

Account-data times, feature status, MOTD, hotfix data, spells, cinematic and
initial object packets can be framed/decompressed without gameplay dispatch.
TCPP's login path does not wait for account-data, feature-status, loading-screen,
realm-split or violence-level acknowledgments. Loading-screen handler is a TODO.
RESUME_COMMS needs no acknowledgment. Time sync is the required reply we send.
Do not implement general movement/control acknowledgments as part of this probe.
SMSG_CHARACTER_LOGIN_FAILED (0x4417) carries one reason byte and fails the probe;
unauthorized GUIDs can instead cause immediate disconnect. EOF, timeouts,
cancellation, malformed packets, bad redirects and repeated redirects fail.
Legacy single-socket world verification is explicitly unsupported by this probe.

Each socket owns its own RC4 and persistent zlib history. Compression uses opcode
bit 0x8000, LE32 uncompressed size and Z_SYNC_FLUSH data. Retain the existing
1 MiB packet / 4 MiB cumulative compression bounds. The login wait handles at
most 512 packets under a 30-second deadline and joins both read goroutines on exit.

## Reference differences

Inherited WotLK world.go uses 0x003D and a full LE64 login GUID on one socket;
that dispatcher is untouched. Clientless agrees with the bit/GUID permutation
but sends optional loading-screen and violence-level packets, then starts its
gameplay/event loops. Those are not required by the local TCPP login handlers.
The inspected Clientless login path does not establish the TCPP instance-link
flow used by this QA configuration. TCPP remains authoritative. No claim of
WowPacketParser agreement is made without the unavailable reference.

Live tests may cause normal QA server login/logout persistence. No direct SQL,
database reset, character creation, server-source or runtime changes are made.
