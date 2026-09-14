# Controlled objective progress: stopped checkpoint, NOT LIVE VERIFIED

## 2026-09-14: fresh-login analysis and isolation preflight

See [the detailed fresh ASSIST analysis](cataclysm-434-fresh-assist-analysis.md).
Construction/restoration prove ASSIST/FOLLOW/command-attack-false at the world
insertion boundary; post-load effects and external events still require bounding.
One confirmed-offline stationary login observed pet F140000400000007, owner 2,
alive 65/65, target 0, no combat; Ghost 56/56, target 0, no combat; quest still 0/6.
No pet-control, movement, selection or attack was sent. Two route-valid candidates
failed isolation (neighbor 13.48 yards away with 15-yard roaming; one path passes
0.62 yards from that neighbor). Two others failed unchanged route guards.

Added read-only isolation reporting and tests; the combat safety predicate was
not relaxed. No complete bounded-assistance model or milestone success is claimed.
The preserved draft remains uncommitted; no tag, merge or push. Current stop reason
is documented isolation failure plus incomplete post-load data-driven certification,
not a claim that absent visible aura means absent hidden aura.

## Read-only server diagnostic attempt: blocked before live memory inspection

2026-09-13 continuation preserved all 13 existing draft files in place. No
client code was changed, no login/combat/control packets were sent, and no
server source, binary, configuration, memory or database was modified. No
commit, tag or merge was created. This attempt did **not** complete the requested
live internal-state diagnostic; stored data must not be presented as live values.

### Facilities and concrete blocker

- Read-only process inspection identified QA authserver PID 3412 and worldserver
  PID 10488, both executing from `C:/CataBotLab/QA/runtime`.
- QA configuration has Console.Enable=1, Ra.Enable=0 and SOAP.Enabled=0.
- Existing `.list auras` reads Unit::GetAppliedAuras, including hidden passive
  auras (cs_list.cpp:472). Its command registration disallows console use and
  requires a player session. CliHandler::isAvailable enforces AllowConsole.
- Read-only cataqa_auth queries found account 2 CATAGHOSTQA with no account_access
  or rbac_account_permissions rows. No permissions were granted. No existing
  command in the audited Commands sources reads the complete requested
  ReactState/CommandState/IsCommandAttack state; pet commands are mutations.
- Installed Windows Kits Debuggers/x64 contains dbgcore, dbghelp, srcsrv and
  symsrv DLLs, but no cdb or windbg executable. Neither was found on PATH.
  Visual Studio is installed, including vsdbg components, but matching TCPP
  native symbols were not found in CataBotLab builds or CataBackups.
- Existing dumpbin inspected the QA worldserver PE without modifying it:
  zero COFF symbols; its sole debug-directory entry is `coffgrp`, with no
  CodeView/RSDS PDB reference. No worldserver.pdb was found. Attaching a debugger
  alone would not provide verified layouts/addresses for these internal fields.
  No debugger was attached and no breakpoint or function evaluation was used.

The missing prerequisite is a usable non-mutating native inspection route with
verified layouts/symbols for this exact running binary (or existing instrumentation
that exposes these fields). Rebuilding/patching to supply it is not authorized.
No offsets or live values were guessed.

### Read-only persisted evidence, not runtime proof

Queries used existing QA connection settings without printing or saving credentials;
only SELECT and SHOW statements were executed. Queries were restricted to cataqa
schemas. Initial schema assumptions produced SQL errors, then schema inspection
was used to correct the reads; no statement mutated data.

- cataqa_characters.characters: Ghost guid=2, race=4, class=3, level=1,
  online=0. Ghost was offline at the query, so there was no current active pet
  session to label as a newly sampled live diagnostic.
- character_spell: no 93321 or 93375 row for guid 2.
- character_aura: no 93321 row for guid 2. Persistence is not a complete runtime
  aura inventory and does not prove HasAura false.
- character_pet: id=4, entry=42718, owner=2, level=1, Reactstate=3 (ASSIST),
  slot=0. This is a persistent pet number, not a newly sampled dynamic GUID.
- cataqa_world.playercreateinfo_cast_spell contains raceMask=8, classMask=4,
  spell=79602 for the Night Elf starting Hunter pet. Its note says Young Boar;
  that label is not used as authoritative creature identity.
- Historical SQL configures Control Pet 93321 at level 10, but the current QA
  trainer_spell table has no direct SpellId=93321 row. The historical row must
  not be claimed as current trainer availability or a proven live root cause.

Player::LoadPet and Pet::LoadPetData load saved pets without requiring the control
aura; Pet.cpp:238 restores the persisted reaction. This explains how pet ownership
and missing learned control can coexist in this implementation. It does not
establish the exact previous live aura map, internal summon slot or command flags.

### Requested live values and decision path

| Requested evidence | Result of this attempt |
|---|---|
| GetPet return and internal pet GUID | NOT SAMPLED / UNKNOWN |
| HasAura(93321) | NOT SAMPLED / UNKNOWN |
| CanControlPet(0) | NOT SAMPLED / UNKNOWN |
| Live ReactState, CommandState, IsCommandAttack | NOT SAMPLED / UNKNOWN |
| IsCommandFollow, IsReturning, IsFollowing, IsAtStay | NOT SAMPLED / UNKNOWN |
| Current dynamic pet GUID, target, combat, alive | No new live sample; prior-session evidence below remains historical |
| Exact cause of missing 0x4114 after 0x4924 | UNRESOLVED; neither gate failure nor session discrepancy directly sampled |

Exact source path remains GetPet() && CanControlPet(0), then PetSpellInitialize
repeats both checks. For class 3, spellId 0 skips the spell-learning switch and
tests !HasAura(93321); if false aura, return false immediately. The warlock class
test does not apply. If the Hunter has the aura, CanControlPet(0) returns true;
there is no extra reaction, command, health or pet-existence condition inside
CanControlPet itself. GetPet separately requires its internal Pet GUID slot,
successful map lookup and player IsInWorld.

Existing Server.log contains messages about SMSG_PET_SPELLS being prevented on a
missing socket. They lack body/request correlation and do not prove that the
pet-info request passed its gate: Player.cpp also sends a GUID-zero pet-bar clear
on pet removal. These log lines cannot resolve the exact missing-response cause.

### Runtime architecture remains unchanged

- **Server knows:** applied aura map, internal summon/map lookup, reaction and
  all CharmInfo flags. This diagnostic did not obtain those live values.
- **Normal protocol exposes:** object ownership/health/target/combat; reaction
  and command in gated SMSG_PET_SPELLS. It does not expose hidden aura 93321 or
  directly serialize IsCommandAttack. Existing SMSG_PET_MODE has no traced sender.
- **Safe inference:** the prior session had the bidirectionally identified owned
  pet F140000400000006, alive 65/65, target 0, out of combat. These facts cannot
  establish passive reaction, clear command-attack, or present-time identity.
- **Unobservable through the traced normal readbacks:** the control aura and
  hidden CharmInfo flags; current reaction/command while the response gate is
  unavailable. No new legitimate readback/unlock route was established.
- **Potential deterministic sequence:** if successfully processed for the correct
  first controlled living pet, PASSIVE performs AttackStop and SetReactState(0);
  FOLLOW stops attack, interrupts non-melee spells, sets FOLLOW, clears
  IsCommandAttack/IsAtStay/IsFollowing and sets IsReturning/IsCommandFollow.
  PetAI::CanAttack then returns IsCommandAttack for passive pets; passive also
  disables OwnerAttacked, OwnerAttackedBy and SelectNextTarget paths.
- **Why it is not yet a guarantee:** HandlePetAction can silently reject failed
  lookup/first-controlled/alive prerequisites; no success acknowledgement proves
  both commands ran. It does not test CanControlPet, but using this asymmetry does
  not unlock PetSpellInitialize or create hidden-state readback. A future design
  would need evidence of accepted ordered commands on the same live pet/session,
  bounded intervening events and all automatic attack paths, plus loss/reconnect
  invalidation. Delivery alone is insufficient evidence of handler acceptance.

No pet actions were sent to test that candidate. The safety predicate remains
unchanged. Objective combat is **not safe to resume under the required proof**.
The remaining blocker is first obtaining the requested actual internal runtime
diagnostic, then establishing a repeatable normal-client safety method. No new
parsing/control-gate code was added, so Go test/build validation was not rerun;
the previous passing validation remains applicable to the preserved code.

## Latest continuation: aura visibility boundary confirmed, UNKNOWN retained

On 2026-09-13 the existing ten-file draft was recovered in place, inspected,
and extended without resets or a replacement worktree. The new
`pet-control-gate` command is strictly read-only, even if a pet response arrives.
It cannot send passive/follow, movement, spells or combat.

### Live result

- Ghost GUID 2: 56/56 health, 100/100 focus, target 0, alive, out of combat.
- Owned Pet GUID **F140000400000006**, entry 42718: owner 2, charmer 0,
  65/65 health, target 0, alive, out of combat, map 1. Ownership checks remain
  bidirectional. The current GUID was discovered dynamically.
- Player SMSG_AURA_UPDATE_ALL **0x6916** was successfully decoded: slot 0
  spell 77442 (flags 0x1B), slot 1 spell 49417 (0x19), slot 2 spell 92549 (0x19).
  SMSG_AURA_UPDATE **0x4707** supplied a slot removal and subsequent updates.
- **Aura 93321 = UNKNOWN, not ABSENT.** It did not appear in the visible
  snapshot, but the local source/data show that this aura is not client-visible.
- One CMSG_REQUEST_PET_INFO 0x4924 was sent; no SMSG_PET_SPELLS response arrived
  within five seconds. No SMSG_PET_MODE was observed either.
- Pet reaction UNKNOWN; command UNKNOWN; SafetyProven false; no control command.
- Quest 28713, creature objective 2031: still **0/6**. No movement, target
  selection, combat, loot, turn-in or credit. Session shutdown completed.

Classification remains **UNKNOWN**. This is a proven protocol-observability
boundary, not a claim that an unobserved aura is absent. No subsequent live
action was taken after this result. No DB, TCPP, production resource, or QA
character ability was altered.

### Exact control-gate path and what is observable

1. `Player.cpp:20404`, Player::GetPet(): obtain internal GetPetGUID summon slot;
   require a Pet high GUID; resolve ObjectAccessor::GetPet on the player's map;
   require player IsInWorld. The live published summon/owner/object links prove
   an owned Pet exists on Ghost's map. The internal summon slot itself is not
   transmitted, so the exact function return is not directly sampled.
2. `PetHandler.cpp:973` (HandleRequestPetInfoOpcode) requires GetPet() and
   CanControlPet(). It does not test pet health, target, reaction or command.
3. `Player.cpp:20707`, Player::CanControlPet(0): class 3 (Hunter) returns false
   exactly if HasAura(93321) is false. Class 9 separately tests aura 93375.
   All remaining classes return true. The nonzero spellId overload used during
   spell learning is **not** the path used by the pet-info request.
4. `Player.cpp:20731`, PetSpellInitialize repeats CanControlPet(), then GetPet(),
   before writing reaction/command in SMSG_PET_SPELLS. All ordinary pet login,
   pet spell/action-bar refresh, request, and spell-side callers share that gate.

Ghost's class is authoritatively 3. Pet existence/ownership/health are observed.
HasAura(93321) is not observable through the received packets; consequently
CanControlPet's boolean and the precise rejecting branch cannot be asserted.
The absence of a reply alone cannot distinguish the internal GetPet lookup from
the hidden-aura gate. This continuation does **not** label either false.

### Why the aura snapshot cannot answer HasAura(93321)

`SpellPackets.cpp:224` serializes packed unit GUID followed by aura entries until
packet end: byte slot, int32 spell ID; zero ID removes the slot. Nonzero entries
have int16 flags, byte caster level, byte applications, optional packed caster
unless AFLAG_NOCASTER (8), two int32 duration values when AFLAG_DURATION (0x20),
and int32 effect amounts when AFLAG_SCALABLE (0x40) and effect bits 0/1/2 apply.
No WotLK layout is used. Clientless agrees on 0x4707 and 0x6916.

`Player::SendInitialPacketsAfterAddToMap` calls SendAurasForTarget(this).
`Player.cpp:23522`, SendAurasForTarget serializes **GetVisibleAuras only** and
does not send a packet at all when that set is empty. UPDATE_ALL is therefore
not a list of all auras considered by Unit::HasAura.

`SpellAuras.cpp:1134`, Aura::CanBeSentToClient permits non-passive auras, area
auras, and four special passive effect types (262, 330, 332, 347).
Read-only inspection of the QA build-15595 DBC files found:

- Spell.dbc row 93321: Attributes = 64 (SPELL_ATTR0_PASSIVE).
- SpellEffect.dbc row 97792: SpellID 93321, effect index 0, effect = 6
  (APPLY_AURA), EffectAura = 4 (DUMMY). No other effect rows for 93321.
- APPLY_AURA is not an area-aura effect and DUMMY is not one of the visibility
  exceptions. No 93321-specific code correction was found in SpellMgr/DBC stores.

Thus a successfully decoded visible snapshot cannot prove this aura absent.
The new slot observer records PRESENT from positive server data; ABSENT is
allowed only for a guaranteed-visible aura after a complete snapshot. For
93321, missing/removal/snapshot-without-entry remains UNKNOWN. DBC data explains
the visibility rule; it is not itself evidence of Ghost's current aura state.

### Readback paths investigated across the complete local source

1. Ordinary pet login and CMSG_REQUEST_PET_INFO: both use the same control gate;
   live initialization and explicit request provided no state response.
2. Other SMSG_PET_SPELLS writers: clear-action-bar packets contain no state;
   possess/charm/vehicle initializers concern different ownership/control
   relationships. Ghost has no such relationship; none is used as a workaround.
3. SMSG_PET_MODE / SPetMode: serializer and opcode registration exist, but no
   actual sender was found. Its raw word has no invented mode mapping.
4. Direct state fields: m_reactState and CharmInfo::_CommandState are ordinary
   server members, not update fields. UNIT_FIELD_AURASTATE is an aura-category
   bitmask, not a set of spell IDs; it cannot expose HasAura(93321).
5. Spell/action-bar refresh calls all route through gated PetSpellInitialize.
   Pet AI attack responses expose targets/actions, not full reaction/command.
6. Aura packets expose visible applications only. Their successful decoding
   resolves parser coverage, but cannot make hidden applications observable.

The source-search results are preserved locally in ignored
`bin/pet-readback-source-audit.txt` and `bin/pet-mode-state-source-audit.txt`.
No legitimate packet path establishing the missing state was found for this
current character under the read-only/no-aura-change constraints.

### Validation and remaining decision

Added `client/aura_control_434.go`, `client/aura_control_434_test.go`, and
`client/objective_transport_434_test.go`. Extended the existing pet observer,
CLI/main dispatch and this checkpoint. Extracted the existing combat cancellation
tick unchanged in behavior so the production callback is directly testable.
All previous intended implementation remains preserved: thirteen intended files
in the uncommitted draft. Integration is not changed.

Passed: gofmt, focused Pet434|Objective434 tests, full go test ./..., diff check,
and experimental build. New encrypted/compressed socket tests exercise actual
continued-session transport for pet/combat cancellation and peer loss. They
verify no pet actions after uncertainty, attack-stop/target-clear on cancellation,
no false PASS, and reader/connection cleanup. After peer loss, delivery of an
attack-stop is impossible; the test requires failure and closure, never success.
These tests use synthetic local sockets, not live combat.

Live evidence: `bin/pet-control-gate-live-01.log`.
Matching executable: `bin/azghost-cata-pet-control-gate-15595.exe`.
SHA256: `8254989F324E3235B6337C98011108EA4F9142D7F6D4D26DC3D710A88E64F9DA`.
Logs contain no credentials. Previous evidence remains intact.

Smallest next architectural decision: whether to permit a read-only server-side
diagnostic that can directly inspect hidden HasAura(93321), the internal pet slot,
and reaction/command, rather than requiring packet-only observability. Such a
diagnostic has not been performed or added. Alternatively, a separately approved
QA setup with legitimate pet-control capability would change the preconditions.
Do not silently add the aura, dismiss the pet, bypass the guard, modify TCPP or
begin combat. No success commit/tag/merge/push is warranted.

## Latest continuation: authoritative pet identity established, state unavailable

The existing draft was recovered in place on 2026-09-13. All six original
intended files were retained; no reset, cleanup, stash, replacement worktree,
commit, tag, merge or feature push was performed.

The new stationary `pet-safety` probe independently identifies the active pet:

| Observation | Server-observed value |
|---|---|
| Ghost GUID | 0000000000000002 |
| Active pet GUID | F140000400000005 |
| Creature entry | 42718 |
| Owner GUID | 0000000000000002 |
| Charmer GUID | 0 |
| Health | 65/65, alive |
| Target GUID | 0 |
| In combat | false |
| Position | (10314.199, 829.0924, 1326.4138), map 1 |
| Reaction | UNKNOWN |
| Command | UNKNOWN |
| Pet state packets received | 0 |
| Pet info request | CMSG_REQUEST_PET_INFO, 0x4924, empty body, once |
| Pet-control commands sent | none |
| Safety proven | false |
| Quest 28713, objective 2031 | 0/6 at probe end |

Ownership proof is bidirectional: the created player's UNIT_FIELD_SUMMON
(0xA/0xB) references this Pet high-GUID object, and its UNIT_FIELD_SUMMONEDBY
(0x10/0x11) references Ghost. The pet also has PLAYER_CONTROLLED (8) and no
charmer. Creature entry, health, target and position come from that object's
server updates. The GUID differs from the previous session; it is never a
hardcoded identity or inferred from proximity/character-screen appearance.

The explicit info request received no authoritative response within five
seconds. Classification: **UNKNOWN**, matching the user's insufficient-state
example. TCPP's HandleRequestPetInfoOpcode calls PetSpellInitialize only when
GetPet() and CanControlPet() are true; Hunter control requires aura 93321.
The observed silence is consistent with that gate, but the probe does not
claim to have independently decoded aura 93321 or a reaction value from it.

No movement, player/pet attack, combat spell, passive/follow command, dismissal,
loot or turn-in was sent. Only session protocol, quest-definition query and the
read-only pet-info request were used. Ghost passed the healthy, out-of-combat
precondition; quest remained 0/6 with no credit events. No target was selected,
no death occurred, and one-credit reconnect verification was not attempted.
Transport shutdown completed. No further live operation followed the UNKNOWN
result. The existing objective controller's pet rejection remains in place.

### Additional source findings

- `Unit.h` distinguishes GetPetGUID (internal summon slot) from GetMinionGUID
  (UNIT_FIELD_SUMMON). `Unit::SetMinion` publishes controllable guardians there;
  this is why the probe additionally requires a Pet high GUID (0xF14), the
  reverse owner link, same map, created unit state and no competing owned unit.
- `Player::PetSpellInitialize`: SMSG_PET_SPELLS **0x4114**, ordinary uint64 GUID,
  uint16 family, uint32 duration, byte reaction, byte command, uint16 flags,
  ten uint32 action-bar words, byte spell count and uint32 packed spell/action
  words, followed by byte cooldown count and cooldown data. A zero GUID-only
  packet clears the action bar; it does not prove the pet is absent.
- `SpellHistory::WritePacket<Pet>` emits six bytes (spell/category) for OnHold,
  fourteen otherwise, without an OnHold discriminator. The parser preserves
  that tail with count/length bounds and does not invent cooldown semantics.
  Reaction/command precede the tail and have unambiguous locations.
- SMSG_PET_MODE **0x2235** has a serializer for uint64 pet GUID + uint32 raw
  mode. No sender/call site was found in this local game source. The parser
  preserves the raw word but never guesses a reaction/command bit mapping.
- CMSG_REQUEST_PET_INFO **0x4924** has no body; it also handles charm/vehicle
  initialization. The probe rejects those unrelated relationships.
- CMSG_PET_ACTION **0x0226**: ordinary uint64 pet GUID, uint32 action word,
  ordinary uint64 target, float32 XYZ. Action type occupies the top byte.
  PASSIVE is `0x06000000`; FOLLOW is `0x07000001`. The narrow serializer emits
  only these actions, with zero target/XYZ; no pet attack/spell/abandon API.
- Reaction values are passive 0, defensive 1, aggressive 2, assist 3. Commands
  are stay 0, follow 1, attack 2, abandon 3, move-to 4. An action-bar button is
  an available action, not the current reaction/command state.
- `PetAI::CanAttack` allows even a passive pet to attack when CharmInfo's
  **IsCommandAttack** is true. That boolean is not in SMSG_PET_SPELLS. Passive
  alone therefore fails this adapter's predicate. FOLLOW clears that boolean,
  stops attacks, interrupts non-melee spells and sets command FOLLOW.
- Defensive/assist behavior depends on OwnerAttacked/OwnerAttackedBy. Aggressive
  pets can select nearby targets. FOLLOW alone is not a safety proof. Passive
  pets return no next target in SelectNextTarget, but explicitly commanded
  attacks remain a separate exception.
- `Pet.cpp` restores saved Reactstate on load and saves it on normal pet save.
  CharmInfo initially uses FOLLOW and clears IsCommandAttack. The probe still
  requires a verified FOLLOW command/readback rather than assuming reconnect
  necessarily constructed a new pet. No persistence mutation was attempted.
- CMSG_PET_STOP_ATTACK **0x6C14** reads a pet GUID and stops an attack; it does
  not establish passive reaction. CMSG_PET_ABANDON **0x0C24** can delete a
  Hunter pet via PET_SAVE_AS_DELETED. Neither is used as a workaround.
- Clientless's build table agrees on 0x4924, 0x4114, 0x2235 and 0x0226.
  No WotLK layout or external parser assumptions were substituted.

### Narrow predicate and control gate

The probe requires source-confirmed ownership; no extra controlled actor;
server-reported PASSIVE/FOLLOW, zero mode flags; living pet with no target or
combat flag; and a FOLLOW reset followed by a new authoritative info response.
A absent-pet result instead requires a created player with no summon/charm and
no contradictory visible controlled unit. Unknown identity/state never passes.

If an initial authoritative info response exists, the tested controller can
send PASSIVE (only if needed) and FOLLOW, then request fresh state. It immediately
invalidates the old state; sending commands never sets SafetyProven. If there
is no initial info response, it sends **no control command** because no supported
readback path is established. This is the branch reached in live QA.

New files: `client/pet_state_434.go`, `client/pet_session_434.go`,
`client/pet_state_434_test.go`, `cmd/azghost/pet_safety.go`. `main.go` adds the
explicit `pet-safety` subcommand; this checkpoint is extended in place.
The original objective files remain intact. Total intended draft: ten files.

Validation passed: gofmt; focused `Pet434|Objective434` tests; `go test ./...`;
`git diff --check`; experimental build. Focused tests cover ownership/identity,
all reaction and command values, malformed/truncated packets, absent readback,
unsafe-state rejection, exact safe command bytes, and the distinction between
sent control and a subsequent authoritative transition. Control transitions
are **synthetically tested only**, not live verified.

Additional ignored artifacts:
`bin/pet-safety-live-01.log` and `bin/azghost-cata-pet-safety-15595.exe`.
Executable SHA256:
`C0DBC603B54690B0609A66E24A890A818CE92782AB86D5B2E4F57DC38F19EDBF`.
No credentials are present in the log. The previous logs/executable remain.

Recommended next work: resolve the legitimate server-observable pet-state
prerequisite. Do not weaken its predicate, bypass the control gate, modify TCPP,
reset QA, dismiss the pet or start combat without new authorization/evidence.
The one-credit milestone and reusable combat engine remain deferred.

## Previous session checkpoint (retained)

Date: 2026-09-13. Branch `codex/cata-434-objective-progress`, based on
`0b36580f8b811609fff00da7404133ab204fa200`. Worktree:
`C:/CataBotLab/CataGhost/bin/worktrees/cata-434-objective-progress`.
Integration remains unchanged. No feature commit, tag, merge or feature push.

## Stop reason

No combat was attempted. The one-credit milestone has **not passed**.

The live player has a summoned pet: UNIT_FIELD_SUMMON (0xA/0xB) is nonzero.
Character enumeration also reports a level-1 pet (display 17090, family 2).
This invalidates the draft adapter's single-player-only combat assumptions.
The second preflight rejects it before movement, with a BAD_TEST diagnostic.

Further read-only source research found:

- Unit::Attack calls controlled AI OwnerAttacked; pet involvement cannot be
  ignored simply because no pet attack packet is sent by the harness.
- PetAI::OwnerAttacked and OwnerAttackedBy depend on reaction state.
- PetAI::SelectNextTarget may select another attacker/owner victim, or an
  aggressive pet's nearby target. Passive and assist states return no next target.
- Player::PetSpellInitialize withholds SMSG_PET_SPELLS unless CanControlPet().
  Hunter CanControlPet checks aura 93321. Neither live preflight received
  SMSG_PET_SPELLS (0x4114). The current observer does not establish that aura
  or the pet's reaction state. Absence of the packet alone is not a reaction value.
- PetHandler::HandlePetAction accepts ordinary GUID, packed action word,
  ordinary target GUID and three floats. ACT_REACTION=6, REACT_PASSIVE=0.
  A passive request has no independently verified response in this adapter.

No pet command was sent and no unavailable ability was assumed. The remaining
pet reaction/control uncertainty is classified **UNKNOWN** and triggers the
user's ambiguous-combat-state / UNKNOWN stop rule. Resume only after explicitly
resolving authoritative pet-state observation and legitimate control or bounded
natural assistance; do not simply remove the pet guard.

## Verified baseline and live observations

Origin hashes were independently read successfully:

- integration: `0b36580f8b811609fff00da7404133ab204fa200`
- prior feature: `6390e9afb5b3c200eada41290c76d6c78d4b4aa3`
- prior annotated tag object: `d071bd44ea4675d99d3215f9cd14995a2fd34a55`
- prior peeled tag: `6390e9afb5b3c200eada41290c76d6c78d4b4aa3`

The existing stationary observer passed two independent logins with quest 28713,
The Balance of Nature, slot 0, incomplete, timer 0, objective entry 2031,
required 6, current 0. Both new preflights also observed 0/6. Ghost was level 1,
race 4/class 3, map 1, health 56/56, primary focus 100/100, out of combat.
Live known-spells packet includes 6603 (Attack), 75 and 3044, among others.
No spell cast is implemented or sent; melee acceptance is not live verified.

First selected candidate: `F13007EF000003C1`, entry 2031, 42/42 HP,
position (10289.6, 828.078, 1334.98), distance 23.2393. Faction template 7:
neutral attackable, neither friendly nor reputation-managed. Other eligible
GUIDs: F13007EF000003C0, F13007EF000003C2, F13007EF000003B4 (all 42/42).

First approach failed endpoint validation before any movement. Requested endpoint
(10292.1, 828.0665, 1334.98), MMap endpoint Z=1333.9297. The returned route also
contains grades above 0.3. Classification: NAVIGATION_ADAPTER_BUG (candidate
selection had not required route validity). The draft now tries candidates in
distance/GUID order and requires the existing route validator before selection.
The second preflight stopped at the new pet guard, so this route-selection fix
has not yet been live exercised. No navigation safeguards were relaxed.

Final observed objective: **0/6**. Target death: none. One-credit persistence:
not attempted. No movement, attack, pet command, loot, acceptance or turn-in
packets were sent by these preflights. Session transports closed normally.
No protected resource, database or TCPP source was modified.

## Protocol research (local authoritative TCPP build 15595)

Sources under `C:/CataBotLab/server/src/server/game`:
Handlers/CombatHandler.cpp, Server/Packets/CombatPackets.cpp,
Server/Protocol/Opcodes.h, Entities/Unit/Unit.cpp, Entities/Unit/UnitDefines.h,
Entities/Object/Object.cpp, Entities/Object/Updates/UpdateFields.h,
Entities/Player/Player.cpp, AI/CoreAI/PetAI.cpp, Handlers/PetHandler.cpp,
Entities/Pet/Pet.cpp, Server/Packets/SpellPackets.cpp.

| Packet | Opcode | Layout |
|---|---|---|
| CMSG_ATTACK_SWING | 0x0926 | ordinary uint64 victim |
| CMSG_ATTACK_STOP | 0x4106 | empty |
| SMSG_ATTACK_START | 0x2D15 | ordinary uint64 attacker, victim |
| SMSG_ATTACK_STOP | 0x0934 | packed attacker, packed victim, int32 NowDead |
| CMSG_SET_SELECTION | 0x0506 | ordinary uint64 target |
| SMSG_SEND_KNOWN_SPELLS | 0x0104 | byte initial; uint16 count; uint32 spell/int16 slot; uint16 history count; 18-byte history entries |
| SMSG_QUEST_UPDATE_ADD_CREDIT | 0x0D27 | existing observer: quest, entry, absolute count, required, victim GUID |

NowDead describes the **attacker**, not the victim. Dead-target, cannot-attack,
range and facing errors are rejection signals, not proof of a kill. TCPP assigns
bad facing 0x6C07 and range 0x0B36; Clientless's table reverses those two labels.
Both are aborts in this draft, and TCPP is authoritative. Clientless agrees on
the four attack-start/stop opcodes. No local WPP implementation was used.

Attack(victim,true) is melee auto-attack; Player::Update drives weapon swings,
not repeated client requests. Ranged auto-shot is a spell flow and is not
implemented. No cast targeting, cast result or GCD machinery is needed for the
proposed melee flow. The server checks living/attackable target, ownership,
visibility, flags and friendliness. Player melee checks 3D range
max(5, playerReach + victimReach + 1.3333334), facing +/-60 degrees unless within
boundary radius, and Unit::AttackerStateUpdate checks LOS. MMaps prove a route,
not LOS. LOS acceptance/damage remains a server responsibility with bounded
timeout; the draft never claims local LOS proof.

Fields: health 0x1A, max health 0x20, power 0x1B+class slot, max power 0x21+slot,
target GUID 0x14/15, flags 0x35, combat reach 0x3C, dynamic flags 0x49.
IN_COMBAT=0x80000; dynamic DEAD=0x20 (alone may represent feign death);
TAPPED=4. Death success requires zero authoritative health, not estimated damage.

Unit::Kill rewards player/group before setDeathState(JUST_DIED), so quest credit
may precede the visible death update. Player::KilledMonsterCredit writes the
quest slot counter and sends absolute AddCredit. The draft immediately stops on
any death/credit/count signal but requires zero target health, the matching
victim credit packet, quest-log 1/6, combat stop and player survival together.
It never increments counters locally. Reconnect uses the existing independent
definition/state equality check. Corpse loot is deliberately deferred.

## Draft implementation and validation

- client/objective_combat_434.go: requirements, candidate/faction/health/range
  validation, start/stop parsing.
- client/objective_session_434.go: read-only definition, existing MMap navigation,
  fixed target combat draft, aborts and one-credit evidence gating.
- client/objective_combat_434_test.go: synthetic protocol/filter/state tests.
- cmd/azghost/objective_progress.go and main.go: explicit experimental command,
  cleanup/reconnect orchestration and sanitized result logging.

gofmt, focused Objective434 tests, go test ./..., git diff --check and the
experimental build passed. This is synthetic/baseline validation, **not live
combat verification**. Cancellation and network-loss behavior still need a
dedicated transport-level combat test before this draft is promoted. The common
session loop closes transports on network failure; delivery of a final stop
cannot be guaranteed after transport loss. Signal cancellation after login is
routed through the combat tick to attempt attack-stop before closure.

Local ignored artifacts in this worktree's bin directory:
starting-quest-state.log, objective-live-01.log, objective-live-02.log,
azghost-cata-objective-progress-15595.exe. Logs contain no account credentials.
Executable SHA256:
`7C04EABA2F323AE39F9A3E68157FA65DC6433EFD0B4871EA23F08ECCE582AED2`.

The feature remains an uncommitted draft. Do not create the success milestone
tag or merge it. Next work is resolving the pet-state blocker and finishing this
one-credit milestone. The reusable basic combat engine remains deferred until
the one-credit milestone actually passes.
