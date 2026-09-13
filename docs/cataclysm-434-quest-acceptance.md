# Cataclysm 4.3.4 / 15595 quest acceptance

This bounded probe discovers a quest in live NPC gossip, queries details when appropriate,
accepts exactly one eligible quest, and independently verifies persistence in
the player's quest-log object fields after reconnect. It never sends objective,
combat, loot, reward-selection, completion, or turn-in requests.

## Authoritative source and protocol

The local TCPP source is the inverse implementation for QA. Research used
`QuestHandler.cpp`, `QuestPackets.cpp/.h`, `QuestDef.h/.cpp`, `Player.cpp/.h`,
`GossipDef.cpp`, and build-specific `UpdateFields.h` and `Opcodes.h` under
`C:/CataBotLab/server/src/server/game`. No server or database edits are needed.

| Message | Opcode | Payload |
|---|---|---|
| Questgiver query/details request | `0x2F14` | ordinary little-endian uint64 giver, uint32 quest ID, uint8 RespondToGiver=false |
| Quest details | `0x2425` | layout below |
| Accept quest | `0x6B37` | ordinary little-endian uint64 giver, uint32 quest ID, **uint32** StartCheat=0 |
| Invalid quest | `0x4016` | uint32 reason |
| Quest log full | `0x0E36` | empty |
| Gossip complete | `0x0806` | empty; not acceptance proof |
| Quest-log updates | `0x4715` | existing UPDATE_OBJECT parser and store |

No bit-packed or XOR GUID sequence is used by these three quest messages.
Clientless's opcode table agrees, but it does not provide a quest conversation
implementation. Inherited AzerothGhost WotLK uses accept `0x0189` and details
`0x0188`; its accept payload has the same uint64/uint32/uint32 shape. WotLK
gameplay code remains untouched. No local WowPacketParser quest module was used;
the exact QA serializer, including its fixed reward arrays, takes precedence.

Gossip quest entries contain ID, icon/type, signed level, flags, repeatable byte,
and terminated title. In TCPP `PrepareQuestMenu`, **type 2** means an offered
quest for a player with QUEST_STATUS_NONE; 4 can indicate incomplete or complete
involved quests; 0 is a turn-in style offer. These are not the bitmask values
of the separate questgiver-status response. Only type 2 is eligible here.

Details field order: giver GUID, informing-unit GUID, quest ID, seven terminated
strings (title, description, objectives, giver portrait text/name, turn-in
portrait text/name), two portrait IDs, auto-launched byte, flags, suggested
party size, StartCheat byte, popup byte, required spell ID, reward structure,
description-emote count, then type/delay pairs.

Rewards: choice count followed by three **fixed six-element** arrays (ID,
quantity, display); guaranteed count followed by three **fixed four-element**
arrays; money, XP, title, unknown uint32, unknown float32, bonus talents,
unknown uint32, faction flags; three five-element faction arrays; completion
display spell and spell; two four-element currency arrays; skill and skill-ups.
Counts do not determine serialized array sizes. TCPP sends four description
emotes. The parser bounds counts and strings and requires exact packet exhaustion.
Quest level comes from gossip; minimum level is not sent in this details packet.

## Validation and acceptance proof

`HandleQuestgiverQueryQuestOpcode` checks the giver relation and CanTakeQuest.
It can automatically add AUTO_ACCEPT quests during a **details request**.
The probe rejects turn-in, recurring, PvP, tracking, phase-changing,
scripted-accept and other non-ordinary flag combinations. AUTO_ACCEPT is allowed
only alongside otherwise permitted flags. For these offers it sends explicit
accept directly, without a details request. TCPP's accept handler does not require
opening details: it validates the giver relation, interaction, CanTakeQuest and
CanAddQuest itself. This avoids acquiring the quest before the explicit packet.
For ordinary offers the default remains details-only; an inspected quest ID
enables acceptance and must match the quest rediscovered in fresh live gossip.

Accept checks giver existence, quest relation, CanInteractWithQuestGiver,
CanTakeQuest and CanAddQuest. For an NPC the interaction check requires living,
uncharmed, non-hostile state and range (combat reach plus 4 units through TCPP's
distance helper). Target selection and an open gossip menu are not acceptance
requirements, but the probe retains both until confirmation. Already accepted,
prerequisite, unavailable, wrong-giver and range failures may return an invalid
reason and/or gossip closure; full logs return the full-log packet. Gossip
closure also happens on success, so it cannot distinguish rejection.

`AddQuest` initializes quest ID, state, counters, timer and emits object changes.
`PLAYER_QUEST_LOG_1_1` is `UNIT_END (0x92) + 0x0B = 0x9D`. TCPP uses 25 slots
despite the client field table reserving 50. Stride is five uint32 fields: ID,
state (complete=1, failed=2), two counter words containing four uint16 counters,
and timer. A created object starts with zero-valued fields; sparse create masks
omit zeros. Quest projections require a created player object and reject
duplicate IDs. Source intent never mutates the object store.

The session sends accept once, waits for the chosen ID in a server-provided
slot, requires exactly one addition while existing quests remain unchanged,
then closes. After the existing 65-second TCPP cleanup interval, fresh login
and object creation must contain the same quest/state/counters/timer. This is
the primary proof; there is no database query for acceptance verification.

## Navigation and scope

The quest approach reuses the verified NPC navigation/session components. The
probe prefers creature entry 2079; reusable code takes this preference as an
argument, and both NPC GUID and quest ID are rediscovered. Already-near NPCs
try deterministic lateral destinations 2.5 units from the NPC submitted to MMaps; no
coordinate teleport or direct fallback is allowed. Existing NPC-only behavior
retains its original distance filter. Candidate routes must pass the existing
0.3 grade limit and all endpoint/length/segment checks, with a projected endpoint
within 3.5 units of the NPC (inside the minimum 4-unit interaction radius).
The conversation hook preserves existing
encrypted reads/writes, compression, world observation and time-sync handling.

Packet details do not expose every possible server script or acceptance source
spell. The details-only run permits reviewing the actual offered quest before
acceptance. No broad claim of arbitrary quest safety is made. There is no
abandon/reset action; a successfully accepted quest stays on Ghost.

## Validation

Synthetic tests cover request byte order and trailing widths, full reward-array
layout, every truncated details prefix, bad counts/identity/trailing bytes,
gossip availability filtering, already-present quests, packed counters, duplicate
log IDs, one-send acceptance, rejection/timeout/send failure, zero acceptance in
preview, a real values-only update, and independent reconnect presence checks.
Earlier NPC/navigation and full session regressions remain required.

Run `quest-acceptance` with the usual explicit QA endpoint/character/data flags.
For ordinary offers, leave `-inspected-quest-id` unset for discovery/details only.
AUTO_ACCEPT offers use one explicit accept directly from live gossip instead.
After inspecting an ordinary live quest, supplying that rediscovered ID enables
exactly one accept request. The diagnostic `quest-reconnect` command requires
an explicit `-inspected-quest-id`; it never assumes a quest ID.
Credentials are supplied transiently through the existing environment mechanism.

## Approach geometry regression

The rejected QA route started at authoritative `(10310.29,830.7877,1326.6477)`.
The NPC was at `(10312.7,830.122,1326.53)`. The original destination was
`(10310.379,829.19354,1326.53)`. FindPath returned smoothed, mesh-height-normalized
points `(10310.29,830.7877,1327.2178)` and `(10310.379,829.19354,1327.1288)`.
The validator deliberately anchors the first segment to the authoritative player,
giving delta `(0.088867,-1.594177,0.481079)`, horizontal `1.596652`, grade
`0.301305`. Mesh start projection differs from authoritative Z by `0.570068`;
the mesh-to-mesh segment itself descends `0.088989`, but replacing the player
anchor would hide a vertical discontinuity. No guard or Z normalization was changed.

Classification: NAVIGATION_ADAPTER_BUG, a single unsuitable destination despite
safe reachable alternatives. The +1.3-radian candidate produces requested
`(10311.414,827.9781,1326.53)`, projected endpoint Z `1326.9685`, delta
`(1.124023,-2.809631,0.320801)`, horizontal `3.026129`, grade `0.106010`.
Both are normal ground-navmesh routes; this evidence does not claim a terrain
height defect. The pathfinder API exposes the final smoothed/normalized points,
not the internal polygon corridor as a second raw coordinate path.
