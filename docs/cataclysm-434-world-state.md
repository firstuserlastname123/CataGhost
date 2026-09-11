# Cataclysm 4.3.4 / 15595 initial world state

This milestone observes the initial snapshot after explicit roster selection and
instance login. It sends no movement, character creation, query, spell, gossip,
combat, or other gameplay action. The only post-selection writes are the existing
login request, instance authentication, and required time-sync replies. Success
requires an enumerated player create, decoded position consistent with
LOGIN_VERIFY_WORLD, and completed login/time sync. The probe observes for one
additional second after that proof, within a 30-second deadline. It does not
claim the visible world is static or that every optional login packet is decoded.

## Authoritative references and conflicts

Read-only local TCPP source revision `aa47817dfb`:

- `Entities/Object/Updates/UpdateData.cpp`, `UpdateData.h`: envelope/types.
- `Entities/Object/Object.cpp`, `BuildMovementUpdate`, `BuildValuesUpdate`:
  create movement, optional blocks, masks and values.
- `Movement/Spline/MovementPacketBuilder.cpp`: create spline bits/data.
- `Entities/Object/Updates/UpdateFields.h`: generated 15595 field indices.
- `Entities/Object/ObjectGuid.h`: type IDs; packed GUID representation.
- `Server/Packets/MiscPackets.cpp`: destroy object.
- `Server/Packets/WorldStatePackets.cpp`: world variables.
- `Server/Packets/MovementPackets.cpp`: NEW_WORLD.
- `Server/Protocol/Opcodes.h`, world socket/packet compression code.
- `Entities/Player/Player.cpp`: initial packets before/after map insertion.
- `Entities/Unit/StatSystem.cpp`, `DataStores/DBCStores.cpp`: class power slots.

AzerothGhost's inherited `client/world.go` uses a WotLK count-first envelope,
different update-type numbers, a uint16 movement flag word, different update
indices, and a separate compressed-update path. None can be reused as a 15595
layout. That file remains unchanged. Clientless declares UPDATE_OBJECT=0x4715
but has no object-update decoder in its `src` tree; it corroborates the opcode,
not the layout. No local WowPacketParser 15595 decoder was found. An attempted
upstream raw-file lookup was unavailable; no WPP corroboration is claimed.
TCPP's inverse writer is authoritative for this QA milestone.

## Wire structures

Existing continuously encrypted realm/instance header readers are retained.
SMSG_UPDATE_OBJECT is 0x4715. Compression sets bit 0x8000, producing 0xC715;
the body starts with LE32 uncompressed size followed by zlib data. TCPP maintains
a connection-specific DEFLATE stream with Z_SYNC_FLUSH. The existing bounded
reader preserves that history separately on each socket (1 MiB packet and
4 MiB cumulative compressed/inflated probe limits). It is not WotLK's separate,
independently compressed update opcode. Uncompressed packets do not reset it.

Inflated/uncompressed body: LE16 map, LE32 block count, then blocks:

| Type | Layout after type byte |
|---|---|
| 0 values | packed GUID, values |
| 1 create / 2 create2 | packed GUID, u8 object type, create movement, values |
| 3 out of range | LE32 count, packed GUIDs |

Normal packed GUID: u8 low-to-high byte presence mask, then present raw GUID
bytes in increasing order. This differs from embedded movement GUIDs, whose
MSB-first mask bits and XOR-1 byte sequences have explicit permutations.
Values: u8 mask-word count, LE32 mask words, then LE32 values for set bits in
ascending field order. All indices are retained, including unknown indices.
Missing fields remain unknown; no guessed values are reported.

Create movement begins with MSB-first bits: hover, suppressed greetings,
rotation, animation kits, combat victim, self, vehicle, living movement,
24-bit pause count, no-birth, GO transport, stationary, area trigger, portals,
server time. Conditional living data has inverted flags/orientation/pitch/time/
elevation presence bits, 30-bit movement flags, 12-bit extra flags, GUID masks,
transport flags/mask, fall direction, and spline bits. Byte bodies follow one
alignment boundary. The implementation follows every source permutation rather
than using WotLK's skip path.

Living position is interleaved Z, X, Y with speeds, GUID bytes, time, optional
transport/fall/spline data; orientation is optional (absence means zero).
GO transport holds relative position, GUID, seat and optional times/vehicle ID.
Stationary position is O,X,Y,Z. Vehicle adds orientation/ID; rotation adds u64;
area trigger adds 16 floats and one byte; victim adds XOR GUID; animation kits
add up to three u16 IDs; server time adds u32. Transport/spline details are
consumed to frame the snapshot; no interpolation or movement simulation runs.

Spline bits: active; if active, mode2, effect-start presence, node-count22,
facing2 (angle0/spot1/target2/normal3), optional target mask, acceleration
presence, flags25. Active data: optional acceleration, elapsed time, optional
angle/target GUID, nodes Z/X/Y, optional facing spot X/Z/Y, duration modifier,
duration, optional effect time, second modifier. All splines end with destination
Z/X/Y and spline ID, including inactive/finalized splines.

SMSG_DESTROY_OBJECT 0x4724: full LE64 GUID and u8 dead flag.
SMSG_INIT_WORLD_STATES 0x4C15: LE32 map/area/subarea, u16 count, pairs of LE32
variable/value. SMSG_UPDATE_WORLD_STATE 0x4816: LE32 variable/value, u8 hidden.
SMSG_NEW_WORLD 0x79B1: float X,O,Z, LE32 map, float Y. A transition invalidates
stale map objects and stops this stationary probe explicitly; transfer
acknowledgments belong to a later milestone.

## Fields and object lifetime

| Object type | ID | End index (exclusive) |
|---|---:|---:|
| Object | 0 | 0x008 |
| Item | 1 | 0x04A |
| Container | 2 | 0x094 |
| Unit | 3 | 0x092 |
| Player | 4 | 0x568 |
| Gameobject | 5 | 0x014 |
| Dynamic object | 6 | 0x00E |
| Corpse | 7 | 0x024 |
| Area trigger | 8 | 0x00E |

Object GUID occupies 0/1, type mask=4, entry=5, scale=6. Unit bytes=0x19
(race/class/gender/power type), health=0x1A, powers=0x1B..1F,
max health=0x20, max powers=0x21..25, level=0x30, faction=0x31,
display=0x3D. Player-specific fields start at UNIT_END=0x92.
Gameobject display=0xA, flags=0xB, parent rotation=0xC..F, faction=0x11.
Item owner starts8; container slots start0x4C. Dynamic caster starts8,
spell=0xB; corpse owner starts8, display=0xC, items=0xD..1F.
Names come from enumeration for the player; nearby entry/display/faction IDs
are retained without sending name/template queries.

Power slots follow ascending DBC row IDs, not the power-type number.
The read-only local `ServerRun-14.44/dbc/enUS/ChrClassesXPowerTypes.dbc` has SHA256
`9379AACE585F12EBBCCDEFB6EBC476EC4DEF529F05412B0F9C6758978A15DB45`.
The build-specific mapping follows those rows and DBCManager's assignment.
Unknown class/power combinations remain unmapped. No runtime file was changed.

The session-owned store maps GUID to type, creation/self status, map, latest
create position, raw known fields, and monotonic update revision. Values-only
updates preserve previous fields and position; an unseen GUID gets a placeholder,
not an invented create. A create replaces stale state. Out-of-range/destroy
remove objects; map change clears them while retaining enumerated player identity.
Snapshots are defensive copies sorted by GUID. Entire packets parse and identity
checks pass before mutation. A separate world-variable map stores initial/updated
values. Other observed packet categories are counted, not interpreted as actions.

## Validation and live command

Tests cover populated player/unit fixtures, rich embedded transport and spline,
nonliving type framing, packed GUIDs, masks, explicit zero values, unknown fields,
atomic failure, malformed counts/truncation, create/value/remove/recreate lifetime,
map changes, identity and position checks, world variables, cancellation/deadlines,
fragmented compressed streams, and encrypted two-socket login with object updates
before verification. Mock servers require the time-sync reply followed by EOF;
any extra gameplay packet fails the test. Previous auth/world/enum/login tests
remain part of `go test ./...`.

Build separately with `go build -o bin/azghost-cata-world-state-15595.exe ./cmd/azghost`.
With credentials supplied securely through transient process environment:

```powershell
./bin/azghost-cata-world-state-15595.exe world-state -auth-server 127.0.0.1:3725 -realm-name 'Trinity QA' -expected-world-address 127.0.0.1:8086 -login-character Ghost -expected-instance-address 127.0.0.1:8087
```

Initial packets also include control/bind/server information, spells/talents,
action buttons, factions, social/account data, currencies, achievements, time,
auras and visibility packets (TCPP before/after-add-to-map functions). Those
categories may be counted without decoding their contents. Player/object/world
updates, verification and time-sync are the proof for this milestone.
Movement is explicitly the next milestone, after preservation and review.
