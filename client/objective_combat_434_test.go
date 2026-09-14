package client

import (
	"encoding/binary"
	"math"
	"reflect"
	"testing"
)

func TestObjective434RequirementsAndHealth(t *testing.T) {
	c, p, n := combatFixture434(t)
	q := c.result.Observation.Quest
	if o, err := singleObjective434(q, 0); err != nil || o.Target != 1234 {
		t.Fatal(o, err)
	}
	for _, count := range []uint32{1, 2, 6} {
		q.Objectives[0].Current = count
		if _, err := singleObjective434(q, 0); err == nil {
			t.Fatalf("accepted starting count %d", count)
		}
	}
	p.Fields[FieldUnitBytes434] = 4 | 1<<8 | 1<<24
	p.Fields[FieldPower1434] = 120
	p.Fields[FieldMaxPower1434] = 1000
	p.Fields[combatFlags434] = 0x80000
	u, err := combatUnit434(p)
	if err != nil || u.Health != 100 || u.Power != 120 || u.MaxPower != 1000 || !u.InCombat || u.Dead {
		t.Fatal(u, err)
	}
	n.Fields[FieldHealth434] = 0
	u, err = combatUnit434(n)
	if err != nil || !u.Dead || u.Health != 0 {
		t.Fatal(u, err)
	}
	p.Fields[combatReach434] = math.Float32bits(2)
	n.Fields[combatReach434] = math.Float32bits(3)
	if r, err := meleeRange434(p, n); err != nil || math.Abs(r-6.3333334) > 0.00001 {
		t.Fatal(r, err)
	}
	p.Fields[combatReach434] = math.Float32bits(float32(math.NaN()))
	if _, err := meleeRange434(p, n); err == nil {
		t.Fatal("accepted NaN reach")
	}
}

func TestObjective434FriendlyReputationNeighbor(t *testing.T) {
	c, p, n := combatFixture434(t)
	c.factions.templates[2][10] = 99
	c.factions.templates[1][1] = 99
	c.factions.factions[100][1] = 0
	if reaction, attackable := c.factions.combatReaction434(n, p); reaction != "friendly" || attackable {
		t.Fatal(reaction, attackable)
	}
}

func TestObjective434SpiritByName(t *testing.T) {
	roster := CharacterRoster434{Characters: []Character434{{GUID: 55, Name: "Ghost", Race: 4, Class: 3}, {GUID: 99, Name: "Spirit", Race: 4, Class: 1}}}
	c, err := selectLoginCharacter434(roster, "Spirit")
	if err != nil || c.GUID != 99 || c.Race != 4 || c.Class != 1 {
		t.Fatal(c, err)
	}
	if _, err := selectLoginCharacter434(roster, "Missing"); err == nil {
		t.Fatal("selected absent name")
	}
}

type objectiveTestFixture434 struct {
	result   struct{ Observation QuestProgressObservation434 }
	factions *NPCFactions434
}

func combatFixture434(t *testing.T) (*objectiveTestFixture434, *Object434, *Object434) {
	t.Helper()
	nav := navFixture434(t)
	p := nav.World.Store.objects[17]
	p.ThisIsYou = true
	p.Fields[FieldHealth434] = 100
	p.Fields[FieldMaxHealth434] = 100
	p.Fields[FieldFaction434] = 1
	p.Fields[questLogBase434] = 900
	n := &Object434{GUID: 0xf130000123000456, Type: 3, Created: true, Map: 1, Position: &Position434{X: p.Position.X + 10, Y: p.Position.Y, Z: p.Position.Z}, Fields: map[uint16]uint32{FieldEntry434: 1234, FieldHealth434: 42, FieldMaxHealth434: 42, FieldFaction434: 2}}
	nav.World.Store.objects[n.GUID] = n
	_, f, _ := npcFixture434()
	f.factions[100][1] = 0xffffffff
	d := QuestDefinition434{ID: 900}
	d.Targets[0] = [4]uint32{1234, 6, 0, 0}
	q, e := ProjectQuestProgress434(d, &nav.World.Store)
	if e != nil {
		t.Fatal(e)
	}
	c := &objectiveTestFixture434{factions: f}
	c.result.Observation = QuestProgressObservation434{World: nav.World, Quest: q}
	return c, p, n
}
func TestObjective434SelectionFilters(t *testing.T) {
	for _, name := range []string{"valid", "wrong entry", "dead", "corpse", "combat", "tapped", "damaged", "friendly", "reputation", "unknown faction", "map", "missing position", "controlled", "nonattackable", "immune", "targeted"} {
		t.Run(name, func(t *testing.T) {
			c, p, n := combatFixture434(t)
			n.Position.X = p.Position.X + 10
			switch name {
			case "wrong entry":
				n.Fields[FieldEntry434]++
			case "dead":
				n.Fields[FieldHealth434] = 0
			case "corpse":
				n.Fields[combatDynamic434] = 0x20
			case "combat":
				n.Fields[combatFlags434] = 0x80000
			case "tapped":
				n.Fields[combatDynamic434] = 4
			case "damaged":
				n.Fields[FieldHealth434]--
			case "friendly":
				c.factions.templates[2][10] = 99
				c.factions.templates[1][1] = 99
			case "reputation":
				c.factions.factions[100][1] = 0
			case "unknown faction":
				n.Fields[FieldFaction434] = 999
			case "map":
				n.Map = 2
			case "missing position":
				n.Position = nil
			case "controlled":
				n.Fields[0x10] = 17
			case "nonattackable":
				n.Fields[combatFlags434] = 2
			case "immune":
				n.Fields[combatFlags434] = 0x100
			case "targeted":
				n.Fields[combatTarget434] = 17
			}
			got := objectiveCandidates434(&c.result.Observation.World.Store, c.factions, 1234)
			if (len(got) == 1) != (name == "valid") {
				t.Fatalf("%s: %+v", name, got)
			}
		})
	}
	c, p, n := combatFixture434(t)
	n.Position.X = p.Position.X + 10
	other := *n
	other.GUID++
	c.result.Observation.World.Store.objects[other.GUID] = &other
	for i := 0; i < 20; i++ {
		got := objectiveCandidates434(&c.result.Observation.World.Store, c.factions, 1234)
		if len(got) != 2 || got[0].GUID != n.GUID {
			t.Fatal(got)
		}
	}
}

func TestObjective434Protocol(t *testing.T) {
	guid := uint64(0xf130000123000456)
	if got := targetGUID434(guid); !reflect.DeepEqual(got, []byte{0x56, 4, 0, 0x23, 1, 0, 0x30, 0xf1}) {
		t.Fatal(got)
	}
	start := binary.LittleEndian.AppendUint64(nil, 17)
	start = binary.LittleEndian.AppendUint64(start, guid)
	e, ok, err := parseCombatEvent434(0x2d15, start)
	if err != nil || !ok || e.Attacker != 17 || e.Victim != guid {
		t.Fatal(e, err)
	}
	stop := []byte{1, 17, 0xdb, 0x56, 4, 0x23, 1, 0x30, 0xf1, 0, 0, 0, 0}
	e, ok, err = parseCombatEvent434(0x0934, stop)
	if err != nil || !ok || e.Attacker != 17 || e.Victim != guid || e.NowDead {
		t.Fatal(e, err)
	}
	for _, tc := range []struct {
		op uint16
		b  []byte
	}{{0x2d15, start}, {0x0934, stop}} {
		for i := 0; i < len(tc.b); i++ {
			if _, _, err := parseCombatEvent434(tc.op, tc.b[:i]); err == nil {
				t.Fatalf("accepted truncation %d", i)
			}
		}
		if _, _, err := parseCombatEvent434(tc.op, append(tc.b, 0)); err == nil {
			t.Fatal("trailing byte")
		}
	}
}
