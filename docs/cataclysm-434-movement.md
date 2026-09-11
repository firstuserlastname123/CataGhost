# Cataclysm 4.3.4 build 15595 controlled movement

The probe performs one approximately one-unit forward displacement using the
enumerated player's initial server position and facing. It does not implement
navigation, pathfinding, combat, interaction, or any AI behavior. The inherited
WotLK `client/world.go` remains unchanged.

## Source research

Authoritative read-only reference: local TCPP `aa47817dfb`.

- `Movement/MovementStructures.cpp`: MovementStartForward, MovementStop,
  MovementHeartBeat, MovementUpdate, MovementSetFacing, MovementStartTurnLeft,
  MovementStartTurnRight, MovementStopTurn.
- `Entities/Player/Player.cpp`: ReadMovementInfo and ValidateMovementInfo.
- `Entities/Unit/Unit.cpp`: WriteMovementInfo and UpdatePosition.
- `Entities/Unit/UnitDefines.h`: movement flags and secondary flags.
- `Handlers/MovementHandler.cpp`: HandleSetActiveMoverOpcode and
  HandleMovementOpcode; `Server/GameClient.cpp`: allowed/active mover state.
- `Server/Packets/MovementPackets.cpp`: SetActiveMover::Read.
- `Server/WorldSession.cpp`: IsRightUnitBeingMoved, disconnect expiry,
  LogoutPlayer and SaveToDB.
- `Server/Protocol/Opcodes.h`: opcode values.

Clientless's opcode table agrees with TCPP but its source has no movement
serializer/decoder implementing this flow. AzerothGhost's WotLK serializer uses
a packed GUID followed by fixed LE32 flags, LE16 secondary flags, timestamp,
XYZ/O and fall time; its start opcode is 0x00B5. That byte layout is incompatible
with Cataclysm. No local WowPacketParser 15595 implementation is available;
this milestone does not claim WPP corroboration. The exact four implemented
token sequences were mechanically compared with TCPP after transcription.

| Message | 15595 opcode | Role |
|---|---|---|
| CMSG_SET_ACTIVE_MOVER | 0x3314 | Establish permitted enumerated mover |
| MSG_MOVE_START_FORWARD | 0x7814 | Begin forward movement at initial position |
| MSG_MOVE_HEARTBEAT | 0x3914 | Halfway position, forward flag |
| MSG_MOVE_STOP | 0x320A | Final position, forward flag cleared |
| SMSG_MOVE_UPDATE | 0x79A2 | Server movement broadcast |
| MSG_MOVE_SET_FACING | 0x7914 | Researched; not sent |
| MSG_MOVE_START_TURN_LEFT / RIGHT | 0x700C / 0x7000 | Researched; not sent |
| MSG_MOVE_STOP_TURN | 0x331E | Researched; not sent |

Each movement opcode has its own ordering. Start places Y,Z,X first; stop
places X,Y,Z; heartbeat places Z,X,Y. The remaining fields mix MSB-first
presence masks and 30/12-bit flags with permuted XOR-1 GUID byte sequences.
Flags, secondary flags, time, orientation, pitch, and spline-elevation presence
bits are inverted. Transport, fall and fall-direction presence are positive.
All scalar byte fields are little endian. Byte reads resume at the next byte
boundary. The server broadcast starts with bits and interleaves Y,X,Z later.
Literal source-derived sequences are in `client/movement_434.go`.

Set-active-mover mask order: 7,2,1,0,4,5,6,3; XOR byte order:
3,2,4,0,5,1,6,7. TCPP begins with no active mover. Merely logging in does not
replace this required client control message. Its GUID comes from enumeration.

Forward is bit 0x1, backward0x2, strafe0x4/8, turn0x10/20, pitch0x40/80,
walk0x100, root0x400, falling0x800/far0x1000, swimming0x100000,
ascending0x200000, descending0x400000, can-fly0x800000, flying0x1000000,
spline elevation0x2000000. Secondary always-allow-pitch is 0x10.
Pitch is sent for swimming/flying or this secondary flag. Fall data has time
and vertical speed; directional falling adds horizontal speed, sine and cosine.
Transport data has its own masked GUID, relative XYZ/O, signed seat, time and
optional time2/vehicle ID. The codec handles these conditional forms; the live
controller rejects transport, falling, spline, swimming/flying, or other
nonzero movement flags instead of attempting those modes.

Timestamps use the same session monotonic clock as required time-sync replies.
TCPP translates them using the measured clock delta (or server-time fallback).
It checks active/allowed mover identity, pending teleport, valid coordinates,
transport offsets and bounds, conflicting flags, aura-dependent flight/hover/
waterwalking, and under-map position. It stores the movement info and calls
UpdatePosition before broadcasting. Stop also refreshes zone and related state.
This harness does not rely on the absence of a speed rejection as proof.

## Bounded movement and server proof

After login verification, time sync and a player object create, the controller
allows the initial stream to settle for one second. It requires map1, positive
known health, ordinary stationary ground state and finite decoded run speed.
The final XY is initial XY plus `(cos(O),sin(O))`; Z and O are unchanged.
The duration is at least 400ms and never faster than the server run speed.
It sends active-mover, start, one halfway heartbeat, and stop. It continues
processing server traffic for one second, then closes both sockets. No
teleport or unrelated gameplay message is used.

The two proposed positions are never written into the authoritative object
store. Server movement updates alone update existing object position/movement
state and revision. Unknown/destroyed GUIDs are not resurrected. Create snapshots
retain flags, secondary flags, timestamp, position, run speed and optional
transport/fall/pitch data, with defensive copies on store snapshots.

TCPP's `SendMessageToSet(&data, _player)` excludes the moving player. There is
no ordinary self-echo or start/stop ACK. Socket survival therefore proves nothing.
The command waits 65 seconds after disconnect: TCPP expires disconnected sessions
after 60000ms and LogoutPlayer(true) saves the character. It then repeats auth,
enumeration, instance login, time sync and initial world-state observation.

PASS requires the fresh LOGIN_VERIFY_WORLD and player create to agree with the
stop-only final coordinate within0.02 units, same identity/map, stopped state,
0.9..1.1 units horizontal displacement, <=0.05 vertical change and unchanged
facing. Only the stop carried that final coordinate, so this is stronger than
observing the halfway heartbeat or a local prediction. No database read/write,
second account, server patch, forced teleport or invented ACK is involved.
Cancellation/transport errors never produce PASS or automatically retry movement.
If an attempt fails after sending, inspect the server snapshot before any retry.

## Validation

Focused tests independently transcribe ground packet fixtures, test optional
transport/fall/pitch/elevation and secondary flags, malformed/truncated packets,
field widths, GUIDs, stopped state, one-unit bounds, and authoritative store
continuity. Encrypted two-connection synthetic sessions cover fragmentation,
persistent compression across login and movement, another time-sync exchange,
the exact outbound opcode allowlist, cancellation, and EOF without extra actions.
All earlier Cataclysm tests remain regressions in `go test ./...`.

With account credentials supplied only through transient process environment:

```powershell
./bin/azghost-cata-movement-15595.exe movement -auth-server 127.0.0.1:3725 -realm-name 'Trinity QA' -expected-world-address 127.0.0.1:8086 -login-character Ghost -expected-instance-address 127.0.0.1:8087
```

The executable is built separately. Sanitized evidence belongs outside Git in
the QA evidence directory. Following live verification, preserve, commit, tag,
merge, regression-test and push to the user's origin fork. Do not begin
navigation/pathfinding as part of this milestone.
