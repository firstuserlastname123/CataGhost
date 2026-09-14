# Spirit controlled combat preflight — BAD_TEST, no combat attempted

2026-09-14. Branch `codex/cata-434-basic-combat`, worktree
`C:/CataBotLab/CataGhost/bin/worktrees/cata-434-basic-combat`, based on clean
integration `0b36580f8b811609fff00da7404133ab204fa200`.

The one-kill milestone has **not passed**. Stop reason is the observed lack of
a route-valid, sufficiently isolated candidate within the existing bounded
40-yard approach. No further live action followed the rejection. This does not
prove there is no suitable target elsewhere in the world.

## Live identity and quest acquisition

Spirit was selected by name through normal enumeration, GUID `0000000000000003`,
race 4 (Night Elf), class 1 (Warrior), level 1, map 1. Initial XYZ/O:
`10311.3, 832.463, 1326.41 / 5.69632`. Health 60/60, primary rage 0/1000 in raw
protocol units, flags 8 (not in combat), target 0. The created object's sparse
zero fields include CHARM, SUMMON, CHARMEDBY and SUMMONEDBY. No visible unit had
a charmer, summoner or creator relationship to Spirit. This verifies observable
relationships, not hidden server internals. Known-spells opcode 0x0104 was
observed but its contents were not retained/decoded by the baseline observer.

Initial quest log was empty. Existing `quest-acceptance` discovered giver
`F130081F0000120E`, entry 2079, and a live type-2 offer for 28713, The Balance of
Nature. The existing AUTO_ACCEPT handling sent exactly one explicit 0x6B37
acceptance, without a details query. The player log gained slot 0, state 0,
counters [0,0,0,0], timer 0. After normal 65-second cleanup, independent reconnect
confirmed that same slot. No database, GM or eligibility changes were used.

Questgiver navigation used four segments and a 3.168931-yard MMap path. Persisted
arrival XYZ/O: `10310.29,829.4593,1327.151 / 4.3880863`. The existing grade and
endpoint guards passed unchanged. Subsequent stationary live definition query
0x0D06 / response 0x6936 derived entry 2031, required 6, current 0. Spirit remained
60/60, out of combat, without observable controlled actors.

## Candidate evidence

All five candidates below were observed alive at 42/42. Two routes passed;
three failed the existing 0.3 maximum grade. No combat target was committed.

| Candidate GUID | Route | Relevant isolation evidence |
|---|---|---|
| F13007EF00001208 | Valid | Nightsaber 11FC is 10.242586 yards away |
| F13007EF000011FC | Valid | Nightsaber 1208 is 10.242586 yards away; route passes 2.308965 yards from it |
| F13007EF00001209 | Rejected | Grade 0.479719 |
| F13007EF0000120A | Rejected | Grade 0.302994 |
| F13007EF00001214 | Rejected | Grade 0.321232 |

1208 position: `(10313.795,804.9469,1328.7682)`.
11FC position: `(10312.072,794.9501,1330.1847)`.
The runtime configuration reads family assistance radius 10, delay 1500 ms,
flee-assistance radius 30. Read-only Hunter research documented relevant QA
spawn-centered wander radii of 15; those database values were not re-queried in
this attempt. Local Creature.cpp CallAssistance/CanAssistTo support the
same-faction assistance risk. Current 10.24-yard separation is insufficient
evidence of isolation over an approach/combat window. No claim is made that a
second creature actually assisted: combat never began.

The ported diagnostic also recorded reputation/unknown neighbors. Its first
live version classified friendly reputation NPCs conservatively as unsupported,
so the overall minimum clearance includes irrelevant friendly units. This was
corrected after the live stop: explicit template friendliness is checked before
the unsupported reputation branch. A focused regression covers it. The stop
decision independently rests on the **known neighboring Nightsabers** above,
not that global minimum or a requirement for an empty world. No live retry was
made after this correction.

The 30-yard snapshot exclusion is a diagnostic heuristic, not a proven future
roaming envelope. It does not authorize combat. A complete movement-aware
isolation model and continuous combat monitoring remain unimplemented.

## Source scope and validation

Individually reviewed/reused generic research: objective extraction; creature
filters; faction reaction; unit health/power/combat projection; melee range;
attack start/stop parsing; route and segment-distance isolation reporting.
No Hunter pet state, pet session, aura, CharmInfo, ASSIST or control-command
code was ported. The owned-pet exemption in the isolation draft was removed.
New code: generic replicated-control rejection and a stationary
`objective-preflight` command. It has no combat or movement sender.

Files: client/controlled_actor_434.go and its test;
client/objective_combat_434.go and its test;
client/objective_isolation_434.go and its test;
cmd/azghost/objective_preflight.go; cmd/azghost/main.go; this document.

Passed: gofmt; focused UncontrolledPlayer434/Objective434/Quest tests;
`go test ./...`; `git diff --check`; experimental build. Focused tests cover
dynamic Spirit selection, sparse/high-word/reverse control links, requirement
extraction and nonzero starting-count rejection, candidate filters/order,
route failure, isolation geometry, friendly reputation neighbors, health/power,
combat flags, range validation and attack packet parsing/truncation. Existing
quest acceptance and reconnect regressions passed unchanged. No new combat
controller/cancellation/network-loss/one-kill tests are claimed: that sender and
state machine were deferred when the live prerequisite failed.

## Evidence and final disposition

Sanitized local artifacts in this worktree's ignored `bin/`:

- spirit-preflight-live-01.log and spirit-preflight.exe: initial stationary state.
- spirit-quest-acceptance-live-01.log: normal acceptance and reconnect proof.
- spirit-objective-preflight-live-01.log: definition, world snapshot, candidate
  and neighbor evidence, BAD_TEST result.
- azghost-spirit-objective-preflight-15595.exe: executable used for isolation
  observation; SHA256 8388F45C39E9F3980BE048B5FD682D5A14A30EB10B90C628FC8C8BE4BD927A93.
- azghost-spirit-objective-preflight-final-15595.exe: final tested build with
  friendly-neighbor correction; not used for another live attempt.
- spirit-preflight-source.patch: final source/test/documentation draft.
- hunter-preservation-before.json: hashes of all 16 modified/untracked Hunter
  files. Final comparison found zero hash changes; status remained identical.

Final observed objective is **0/6**, not 1/6. No attack start/stop was sent, no
target death or AddCredit occurred, and no one-credit persistence check was
possible. No loot, turn-in, second kill, spell, resurrection or corpse recovery
was attempted. Connections closed normally. Only legitimate quest acceptance
and the verified questgiver movement changed the QA fixture.

No feature success commit, milestone tag, integration merge or origin push.
Integration remains clean at 0b36580. Hunter draft remains preserved at its
original path and branch. No protected database, server source/runtime or
production installation was modified. Nothing was pushed upstream.

Next prerequisite: resolve route/isolation suitability for this first controlled
kill without weakening navigation guards or manufacturing the fixture. The
requested reusable basic combat engine milestone remains appropriate **after**
a future independently verified 0/6 to 1/6 PASS, before Hunter integration.

## Bounded scouting development checkpoint

The next code-only checkpoint adds generic non-combat scouting rather than
changing the failed fixture or lowering navigation/isolation thresholds. When
the current snapshot has no eligible objective, the client considers at most
four observation points, in deterministic forward/left/right/back order, at a
12-yard radius. Every point requires a normal MMap path, the existing 0.3 grade
limit, bounded path length and segment count, and more than the configured
10-yard assistance clearance from known attackable units at the endpoint and
along the route after the unavoidable first four yards of departure.

After each completed scout move the command reconnects normally, rebuilds the
live object store, reprojects quest progress, and reruns candidate isolation.
It performs at most four moves/five observations and returns `BAD_TEST` when
that budget or the available safe points are exhausted. The scouting API has
no selection or attack callback, so it cannot start combat or proceed to a
second target. Candidate ordering remains isolation-first, with shorter route
length and GUID used as deterministic tie breakers. The 30-yard combat
snapshot threshold remains unchanged and intentionally accounts conservatively
for the researched 15-yard wander and 10-yard assistance assumptions.
