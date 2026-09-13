# Cataclysm 4.3.4 / 15595 quest objective observation

This stationary probe selects an active quest from the server's player fields,
requests its protocol definition, projects observed objective progress, and
reconnects to independently compare the definition and state. With no explicit
`-inspected-quest-id`, exactly one active quest is required. No quest identity,
target, or required count is hardcoded in the runtime.

## Authoritative evidence

Read-only local TCPP sources under `C:/CataBotLab/server/src/server/game`:

- `Server/Packets/QuestPackets.cpp`: QueryQuestInfoResponse, AddCredit,
  AddPvPCredit, UpdateComplete, FailedTimer and QuestGiverQuestFailed writers.
- `Handlers/QuestHandler.cpp`: HandleQuestQueryOpcode only returns the definition
  from the loaded quest template. It does not run giver-query auto-accept logic.
- `Quests/QuestDef.cpp`: BuildQueryData copies loaded TDB requirements into the
  response, including target order, counts, item requirements and objective text.
- `Entities/Player/Player.cpp`: quest slot packing; KilledMonsterCredit,
  KillCreditGO, TalkedToCreature, ItemAdded/RemovedQuestCheck,
  AreaExploredOrEventHappens, CanCompleteQuest, CompleteQuest, FailQuest and
  reconnect loading. GetItemCount(entry,true) includes bank inventory.
- `Entities/Player/Player.h`, `Entities/Object/Updates/UpdateFields.h`,
  `Quests/QuestDef.h`, `Server/Protocol/Opcodes.h`: constants and field indices.

The live definition is TCPP's serialization of its currently loaded TDB row.
No direct database access or changes are used. Clientless's 15595 table agrees
on the query and progress opcodes, but supplies no matching objective-state
implementation. Its ADD_KILL name is narrower than TCPP's ADD_CREDIT: the latter
also reports gameobject/cast/talk credit. Existing AzerothGhost WotLK gameplay
does not provide this build-specific observer and remains untouched. No matching
local WowPacketParser quest module was found; no retail layout was assumed.

## Wire and state

| Message | Opcode | Layout / significance |
|---|---|---|
| CMSG_QUERY_QUEST_INFO | 0x0D06 | uint32 quest ID, little endian; read-only |
| SMSG_QUEST_QUERY_RESPONSE | 0x6936 | complete fixed-array definition described below |
| SMSG_QUEST_UPDATE_ADD_CREDIT | 0x0D27 | uint32 quest, target, absolute count, required; ordinary uint64 victim GUID |
| SMSG_QUEST_UPDATE_ADD_PVP_CREDIT | 0x4416 | uint32 quest, absolute count, required |
| SMSG_QUEST_UPDATE_COMPLETE | 0x2937 | uint32 quest; also exploration/event credit, not conclusive overall completion |
| SMSG_QUEST_UPDATE_FAILED_TIMER | 0x6427 | uint32 quest |
| SMSG_QUEST_GIVER_QUEST_FAILED | 0x4236 | uint32 quest, reason |
| UPDATE_OBJECT | 0x4715 / compressed 0xC715 | existing object-store parser; authoritative slot and inventory changes |

The opcode table contains 0x6324 (QUEST_UPDATE_FAILED) but local source has no
serializer/sender; no layout is invented. ADD_ITEM is assigned 0x0000 and has no
implemented sender. Item progress is projected from inventory/bank item fields,
not an invented item message or the four creature counters. 0x55A4 is the
questgiver **reward** notification and must not be confused with objective
completion. Unhandled opcodes remain counted by the existing world observer.

The query response starts with 31 metadata words, four reward item pairs, six
choice item pairs, three five-word faction arrays and four POI words (70 words
total). These are preserved verbatim, including float bit patterns and unmodeled
requirements/rewards. Five terminated strings follow. Then: four tuples of
encoded NPC/GO ID, count, item-drop ID and quantity; six required item pairs;
required spell; four objective strings; four reward and four required currency
pairs; four portrait strings; two sound IDs. Strings are bounded; truncated,
trailing and invalid-ID packets fail. GO IDs use bit 31; raw tuples are retained.

Player quest slots start at 0x9D (UNIT_END 0x92 + 0x0B). TCPP uses 25 slots even
though the client field table reserves 50. Each slot has five uint32 words:
quest ID, state bits, two packed-counter words, timer. Counters are four uint16
values in order; they refer to the four NPC/GO objectives. PvP uses counter 0.
State 1 means complete, 2 failed; unknown/conflicting bits remain unknown.
The timer is the absolute server expiration in seconds, not remaining ms;
failure may set it to sentinel 1. Reconnect restores slots and counters from
server persistence. Sparse created-object fields omit zero values.

## Separation and limits

Definitions, projected progress and explicit notifications are separate values.
Receiving credit never locally increments a slot; values-only object updates
change it. Repeated absolute credit packets therefore cannot double-count.
Reaching an objective count does not mark the whole quest complete. The overall
status comes from player state bits, respecting other server requirements.

Creature-credit is intentionally generic: the protocol target tuple alone cannot
distinguish all kill, talk, cast or scripted-credit mechanisms. Event/exploration
has no individually replicated explored bit. It stays unknown until the whole
quest is complete; notifications are retained independently. Required spell,
currency and otherwise unknown objective types preserve their metadata and
report unavailable current values. Other unmodeled metadata remains in Prefix.

Item counts traverse player equipment/backpack/bank GUIDs (0x1C0, 74 slots),
and container references (slot count 0x4A, GUIDs 0x4C). Item owner is 0x08/09;
stack count is 0x10. Missing referenced objects, invalid ownership or cyclic
references make the count unknown. Counts reflect server inventory, including
removal, rather than guessed quest-status internals. Bank objects may be absent
until observed; this limitation is explicit.

The existing session loop handles encrypted framing, persistent compression,
redirect/instance authentication, time synchronization and clean closure. The
only new outgoing packet is the definition query. The existing 65-second cleanup
wait precedes a fresh login/query; the probe fails if identity/map, definition,
slot, counters, timer or projected state differ. It makes no objective action.

## Live QA definition and scope

TCPP's live response identifies Quest 28713, **The Balance of Nature**, level 2,
minimum level 1, flags 0x80000. Its summary says "Kill 6 Young Nightsabers."
Objective index 0 requires creature credit for entry 2031, count 6. The other
three target slots, six item requirements, currencies and required spell are
zero. Ghost initially has slot 0, incomplete state 0, all counters 0, timer 0.
The objective is therefore 0/6, incomplete.

A live nonzero delta requires killing a creature. Combat is not implemented by
this milestone. No combat, loot, acceptance, abandonment, turn-in, movement or
GM operation is sent. Nonzero live progress is deliberately deferred to **basic
combat plus one quest kill credit**. Synthetic tests cover partial/full counts,
notifications, failures, items, events, truncation, continuity and reconnect.
