# Cataclysm 4.3.4 / 15595 MMap navigation

This milestone connects the inherited AzerothGhost MMap pathfinder to the
verified Cataclysm movement session. It requests a short route, executes it in
small segments and independently verifies the server's saved position. It does
not enable NPC interaction, combat, exploration, swimming, jumping or transport.

## Existing components inspected and reused

`pathfinding.MMapManager` lazily loads a map's 28-byte `.mmap` parameters and
grid tiles. Grid filenames use `(32-X/533.3333,32-Y/533.3333)` and a three-digit
map ID. `PathFinder.FindPath` loads the relevant tile region, projects endpoints
onto Detour polygons, performs the polygon path query and smooths in 4-unit
steps. Game XYZ converts to Detour YZX and back. Smoothed point heights come
from the mesh's detailed polygons. Existing optional terrain/vmap managers can
provide height, but their AzerothCore format assumptions are not needed here.

`navigation.NewEmbeddedNavigator` wraps a pooled pathfinding service, but its
`Found` boolean discards some PathType detail. This adapter uses the same
PathFinder directly so it can reject every non-normal result, including
incomplete, truncated and `NotUsingPath` results. Missing tiles can otherwise
return a two-point straight-line fallback. This milestone never executes that
fallback. `gamedata` explicitly targets WotLK gameplay and is not required.

`movement.MovementController` provides a reusable timer/waypoint controller,
but its current sender interface has void methods, includes jump/fall handling,
and its package imports `client`. Importing it back into `client` would create a
cycle; its default waypoint tolerance is also 1.5 units. `bot.MoveTo` includes
direct-line fallbacks and broader gameplay behavior. A small session-owned
Cataclysm executor therefore consumes the inherited path points directly,
preserves send errors, and uses the existing `encodeMovement434` implementation.
No path-search algorithm or packet serializer is duplicated. WotLK `world.go`
and the existing movement controller are untouched.

## MMap compatibility and ground restrictions

Read-only authoritative TCPP references:

- `src/common/Collision/Maps/MMapDefines.h`: v14, exactly20-byte tile header,
  ground1, steep2, water4, magma8.
- `src/common/Collision/Management/MMapManager.cpp`: tile header/payload loading.
- `dep/recastnavigation/Detour/Include/DetourNavMesh.h`: version7, 64-bit refs.
- `src/server/game/Movement/PathGenerator.cpp`: filter conventions.
- The already documented movement handler/session references remain unchanged.

The inherited default loader expects AzerothCore v19 and56-byte headers.
The QA tile `0011230.mmtile` was inspected: v14, Detour7,20-byte outer header.
The installed detour-go dependency also uses Detour7 and64-bit references.
This is an adapter format mismatch, not missing/corrupt MMaps. No regeneration
or runtime modification is necessary. `NewCataclysm434MMapManager` explicitly
selects the TCPP format; `NewMMapManager` retains its default behavior.

The new mode checks version, file/payload size and Detour section lengths before
unsafe tile views. The private mesh disables off-mesh polygons in memory only.
The query includes only ground1 and excludes all other flags, preventing steep,
water, magma and scripted/off-mesh traversal. QA runtime MMaps are read through
their existing junction; no files under ServerRun-14.44 are written.

Clientless has no navigation implementation to adapt; its opcode table was
already corroborated during movement research. TCPP supplies the MMap and
protocol contracts. AzerothGhost supplies the actual path search and smoothing.

## Request, route and execution

The player is selected by live enumeration. The initial map, position, facing,
health and run speed come from verified login/object state. A candidate seven
units ahead is submitted to `RouteFinder434.FindPath(map,start,destination)`.
That candidate is not itself a route. Execution requires a normal MMap result
with2..74 finite points, length4..15 units, initial endpoint within1 unit of the
server position, and final endpoint within0.75 units of the requested point.
The final tolerance allows small mesh height projection differences.

The raw route is preserved unchanged for reporting. Execution anchors its first
point at the server position instead of teleporting to the projected mesh start.
Subsequent edges are subdivided to at most1 unit, with slope <=0.3 and at most24
segments. Facing is atan2(dY,dX), normalized to0..2pi. Each segment sends the
already verified start/halfway-heartbeat/stop sequence, with duration >=400ms
and no faster than the decoded run speed. Active-mover is sent once. Explicit
stops allow facing to change at subsequent starts without adding new opcodes.

MMap querying runs asynchronously while the session continues reading packets
and replying to time sync. The executor uses the session's monotonic clock and
single writer, preserving encryption/compression continuity. Context cancellation,
the30-second session deadline, pathfinder errors and send errors terminate the
attempt without claiming success. No movement is retried automatically.
Commanded positions remain separate from the authoritative object store.

TCPP excludes the mover from ordinary movement broadcasts. After clean socket
closure, the command waits65 seconds for the source-defined60-second cleanup,
then repeats authentication/enumeration/login/world-state observation. PASS
requires the same character on map1, agreeing LOGIN_VERIFY_WORLD/object/movement
positions, stopped movement state, distance <=0.75 from the requested destination,
distance <=0.05 from the final stop, and matching final facing. Local predictions
or socket survival cannot satisfy this proof.

## Tests and command

Focused tests cover tile format/bounds, default-loader isolation, optional real
MMap loading, request construction/map/current position, multiple points/segments,
route ordering/bounds/slope, no-path/fallback/trivial paths, orientation, send
allowlists, cancellation/deadlines, pathfinder/send error propagation, object-store
continuity and independent final-state tolerances. Existing navigation, pathfinding,
movement, encrypted-session and all earlier Cataclysm tests remain regressions.

Optional read-only actual data test:

```powershell
$env:CATAGHOST_MMAP_TEST_DIR='C:\CataBotLab\QA\runtime\mmaps'
go test ./pathfinding -run TestCataclysm434RuntimeRoute -count=1 -v
```

With credentials supplied only through transient process environment:

```powershell
./bin/azghost-cata-navigation-15595.exe navigation -auth-server 127.0.0.1:3725 -realm-name 'Trinity QA' -expected-world-address 127.0.0.1:8086 -login-character Ghost -expected-instance-address 127.0.0.1:8087 -data-dir C:\CataBotLab\QA\runtime
```

Sanitized evidence records the raw path, length, executed segment count, initial,
requested, commanded and fresh server coordinates, tolerances and result. After
live verification, preserve/commit/tag/merge/test/push to origin; do not proceed
to NPC targeting or interaction until this milestone is reviewed.
