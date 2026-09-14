# Fresh login and natural ASSIST: stopped before combat

2026-09-14. This is source analysis plus one read-only stationary preflight,
not a successful objective attempt or a measurement of hidden server fields.
The existing draft and its safety predicate remain intact. No pet-control,
movement, selection, attack, loot or quest mutation was sent.

## Exact construction and load order

All source paths below are relative to `C:/CataBotLab/server/src/server/game`.

1. `Server/WorldSession.cpp:534`, LogoutPlayer: RemovePet(PET_SAVE_LOGOUT)
   precedes player cleanup/deletion. RemovePlayerFromMap(..., true), SetPlayer(null)
   and only then CHAR_UPD_ACCOUNT_ONLINE mark the character offline. An old socket
   closing alone is insufficient proof of this lifecycle boundary.
2. `Handlers/CharacterHandler.cpp:798`, HandlePlayerLogin: constructs **new
   Player(this)** at line 802. At 972 it calls LoadPetsFromDB; at 975 LoadPet.
3. `Entities/Player/Player.cpp:17995`, LoadPetsFromDB copies saved Reactstate,
   identity, slot, health and action-bar data into PlayerPetData. LoadPet at
   17983 checks IsInWorld and constructs **new Pet(this)**; it calls
   LoadPetData(this, 0, 0, true), deleting the new object on failure.
4. `Entities/Pet/Pet.cpp:40`: Pet constructs Guardian(nullptr, owner, true).
   `Entities/Creature/TemporarySummon.cpp:303,381`: Minion stores m_owner;
   Guardian's properties-dependent InitCharmInfo branch is skipped because
   properties is null. Pet sets its controllable-guardian mask and calls
   InitCharmInfo itself. `Unit.cpp:9588` allocates new CharmInfo when absent.
5. `Entities/Unit/CharmInfo.cpp:24`: initializes **COMMAND_FOLLOW**, and
   **IsCommandAttack=false**. IsCommandFollow, IsAtStay, IsFollowing and
   IsReturning also start false. Constructor temporarily sets creature reaction
   PASSIVE while remembering the old reaction; this is not the final saved state.
6. `Pet.cpp:99`, LoadPetData sets m_loading=true, selects saved pet data,
   validates type/tameability/current-load eligibility, generates a fresh map Pet
   GUID and calls Create. Template initialization can initialize reaction here;
   this occurs **before** saved reaction restoration. It assigns type, faction,
   pet number, model, stats and position.
7. `Pet.cpp:238`: **SetReactState(playerPetData->Reactstate)**. Saved 3 therefore
   becomes ASSIST at this statement. Health is restored next.
8. `Pet.cpp:272`: owner->SetMinion(this,true), **after** saved reaction restoration.
   `Unit.cpp:5692` sets owner GUID, inserts the controlled unit, sets
   PLAYER_CONTROLLED, assigns the internal Pet GUID and publishes UNIT_FIELD_SUMMON.
9. `Pet.cpp:273`: map->AddToMap. `Pet::AddToWorld:65` registers the Pet in the map
   store, calls Unit::AddToWorld and then AIM_Initialize. Thus AI initialization
   occurs during insertion, after the object is registered/in world.
10. `Creature.cpp:939`, AIM_Initialize creates AI and calls InitializeAI.
    `AI/CreatureAISelector.cpp:81` unconditionally selects **PetAI** for IsPet,
    before creature ScriptName/AIName. `PetAI.cpp:47` constructor updates allies;
    inherited `UnitAI::InitializeAI` calls the empty Reset for a living PetAI.
    It does not replace ASSIST/FOLLOW with aggressive behavior.
11. At the end of Pet::AddToWorld, FOLLOW causes all five CharmInfo booleans to
    be reset false again. Therefore at this boundary, the ordinary saved-ASSIST
    load has **ASSIST + FOLLOW + IsCommandAttack=false**.
12. Loading is **not finished at world insertion**: InitTalentForLevel,
    _LoadAuras, LoadPetActionBar, _LoadSpells, passive/level-up spell learning,
    CastPetAuras and scaling auras run afterward. LoadPetActionBar at
    CharmInfo.cpp:236 changes button entries, not _CommandState or command attack.
    Pet::Update returns while m_loading=true (Pet.cpp:523); m_loading clears at
    the end of LoadPetData. Later movement updates may change Following/Returning;
    those flags must not be asserted permanently false from construction.
13. CharacterHandler also has later at_login reset and first-login spell branches.
    These are not universally inert. This QA read found at_login=0 and no saved
    pet_aura rows. Saved pet_spell rows were 2649 and 16827 active=193, and
    65220 active=1. Full certification of every data-driven passive/scaling effect
    was not completed because the independent isolation prerequisite failed.

**Proof boundary:** allocation, restoration, FOLLOW and command-attack reset are
source-proven transitions, not guesses from persisted reaction. The unconditional
claim “every fresh login remains in exactly that state until combat” is too broad:
post-insertion aura effects and subsequent external events must also be bounded.
For example SpellAuraEffects.cpp's feign-death application can set reaction PASSIVE;
the absence of saved pet auras rules out that saved-aura input here, but does not
replace a complete audit of every newly cast aura. No full runtime safety proof
or automatic authorization token was created from this partial boundary proof.

## CanControlPet does not disable PetAI

Player.cpp:20707 with spellId=0 tests Hunter HasAura(93321); a missing aura returns
false. No PetAI path calls this predicate. Its callers gate pet-bar initialization
and spell-learning refresh, not natural assistance. HandlePetAction itself checks
lookup/first-controlled/liveness/CharmInfo; it does not call CanControlPet.
Therefore “missing aura means the pet cannot attack or cannot process commands”
is not a valid safety argument. No commands were used to exploit that asymmetry.

## ASSIST targeting, including exceptions

| Path | Source behavior |
|---|---|
| UpdateAI:85 | Without a victim, scanning is attempted only for AGGRESSIVE or IsAtStay. ASSIST/FOLLOW normally returns to owner. |
| SelectNextTarget:381 | Immediately returns null for ASSIST and PASSIVE, before pet/owner attackers, owner victim and nearby hostile scans. |
| OwnerAttacked:360 | ASSIST may AttackStart the supplied owner-attack target; PASSIVE and DEFENSIVE return. This is actual owner attack, not arbitrary selection. |
| OwnerAttackedBy:340 | ASSIST and PASSIVE return; owner damage alone does not trigger their assistance here. |
| PetAI.h DamageTaken | Calls AttackStart(attacker), including for ASSIST. A second external attacker can matter. |
| AttackStart:318 | Rejects null/self and refuses switching away from an existing living victim; otherwise calls _AttackStart/CanAttack. |
| CanAttack:538 | ASSIST has no special same-owner-target allowlist. Returning/follow/stay flags and existing victim influence acceptance. FOLLOW when not returning can accept a supplied different target. |
| DoAttack:468 | Attacks the supplied target, preserves existing command-attack while clearing other flags for chase; does not invent a different victim. |
| KilledUnit:297 | Stops attack/casting; SelectNextTarget returns null for ASSIST, then return movement clears command flags. No autonomous chain selection. |
| MoveInLineOfSight | PetAI.h overrides it with an empty handler; ordinary creature proximity aggro is not inherited. |
| Autocast in UpdateAI | Negative spells use current victim. Positive spells may inspect pet/owner attacker-for-helper, then allies. Target selection is not a universal proof against secondary spell effects; spell data also needs bounding. |
| Client command/spell paths | Can set command attack true; prohibited entirely for this design. No such packet was sent. |

Thus the audited core **does not independently scan for an uninvolved second
creature in ASSIST**. It can engage a different unit following an external event,
especially damage to the pet. A bounded test must prevent or abort those events;
ASSIST alone is not sufficient. No statement here treats the pet as harmless.

## Actual fresh-login preflight and isolation rejection

Immediately before login, a read-only QA query verified Ghost guid=2, online=0,
at_login=0. Earlier reads found level=1, saved pet number=4, entry=42718, owner=2,
type=HUNTER_PET, slot=0, reaction=3, summon spell=79602, no saved pet auras.

The stationary login dynamically observed Pet **F140000400000007**, owner 2,
charmer 0, 65/65 health, target 0, out of combat. Ghost was 56/56 health,
100/100 focus, target 0, out of combat. Quest 28713's derived entry 2031 remained
exactly 0/6. One pet-info query 0x4924 was sent, no pet-control command. Missing
0x4114 remained UNKNOWN in the existing observer; it was not relabeled as a
measurement of the source-derived state.

Read-only inspection found QA assistance radius=10 yards, delay=1500 ms;
flee-assistance radius=30 yards. Relevant creature rows have MovementType=1 and
wander_distance=15. RandomMovementGenerator::SetRandomLocation samples within the
wander radius and validates movement. Neighbors can therefore enter assistance
range; current separation greater than 10 is not sufficient over a combat window.
No claim is made that every neighbor necessarily will assist or that the 30-yard
flee path is necessarily active for this template.

| Candidate GUID | Route / nearby Nightsaber evidence |
|---|---|
| F13007EF00000E76 | Route valid. E6A is 13.48 yards from target and 15.99 from path; E6B is 21.40 from target. |
| F13007EF00000E6A | Route valid. E76 is 13.48 from target and only **0.62 yards from proposed path**; E6B is 16.63 from target. |
| F13007EF00000E77 | Rejected: route endpoints do not match request. |
| F13007EF00000E78 | Rejected: grade 0.306253 exceeds unchanged 0.3 limit. |

The E76 position matches saved spawn 313142 at (10303.9,805.248,1330.39);
the adjacent position matches spawn 313298 at (10302.3,792.08,1332.79). Both
are entry 2031 with a 15-yard wander radius. Dynamic GUIDs and persistent spawn
IDs are kept distinct; these mappings are position comparisons, not a transmitted
spawn-ID assertion. The potential assistance overlap is sufficient to reject
the bounded test. Unknown/reputation units are also recorded conservatively,
but are **not** the sole reason for rejection; the known Nightsabers suffice.

No target was selected. No movement or combat attempt began. No death, credit,
1/6 result, or 1/6 persistence exists. This was one read-only preflight; it did not
consume the authorized one-combat-attempt budget by actually fighting.

## Implementation and validation

Added a read-only `InspectObjectiveIsolation434` utility, invoked by
`pet-control-gate` when an explicit data directory is supplied. It derives the
entry from the quest, preserves existing route guards, ranks clearance and records
neighbor distances around player/target/all route segments. The conservative
30-yard envelope covers the observed 15-yard wander plus 10-yard assistance with
margin. It is scoped QA policy, not a universal aggro radius or future-world proof.
It cannot enable movement or combat and does not change the safety predicate.

Tests cover isolated/near-player/near-target/near-path candidates, unknown faction,
invalid route and segment geometry. Focused Pet434|Objective434 tests, all Go tests,
gofmt, diff check and experimental build passed. Existing cancellation/network-loss
tests remain passing. The requested full bounded-assistance combat model and its
new monitoring tests were **not implemented or claimed tested**: the independent
live isolation prerequisite failed before enabling that model.

Live artifact: `bin/fresh-assist-isolation-live-01.log` in this worktree. The probe
executable used had SHA256
`5D4660EA2BEC7C00EF912E3073E9FB6D825AD9EC4021E605618340146F1D133E`.
Its raw route-unavailable path distances use MaxFloat64; the final draft reports
-1 for those unavailable path distances. This formatting change was not live rerun.

**STOP / UNKNOWN:** no isolated candidate under the bounded policy, and no complete
post-load data-driven safety certification. Do not relax the predicate or promote
the source boundary to a blanket live-state claim. No TCPP/DB changes, debugger,
reset, pet dismissal, commit, tag, merge, push or reusable combat-engine work.
