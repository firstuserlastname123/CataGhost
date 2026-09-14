package client

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func combatFixture434(t *testing.T) (*objectiveCombatController434, *Object434, *Object434) {
	t.Helper()
	nav := navFixture434(t)
	p := nav.World.Store.objects[17]
	p.Fields[FieldHealth434] = 100
	p.Fields[FieldMaxHealth434] = 100
	p.Fields[FieldFaction434] = 1
	p.Fields[questLogBase434] = 900
	p.Fields[questLogBase434+1] = 0
	p.Fields[questLogBase434+2] = 0
	p.Fields[questLogBase434+3] = 0
	p.Fields[questLogBase434+4] = 0
	p.Movement.Flags = 0
	n := &Object434{GUID: 0xf130000123000456, Type: 3, Created: true, Map: 1, Position: &Position434{X: p.Position.X + 2.5, Y: p.Position.Y, Z: p.Position.Z}, Fields: map[uint16]uint32{FieldEntry434: 1234, FieldHealth434: 42, FieldMaxHealth434: 42, FieldFaction434: 2}}
	nav.World.Store.objects[n.GUID] = n
	_, f, _ := npcFixture434()
	f.factions[100][1] = 0xffffffff
	d := QuestDefinition434{ID: 900}
	d.Targets[0] = [4]uint32{1234, 6, 0, 0}
	q, err := ProjectQuestProgress434(d, &nav.World.Store)
	if err != nil {
		t.Fatal(err)
	}
	nav.Commanded = *p.Position
	nav.Requested = point434(*p.Position)
	nav.Executed = 2
	nav.Segments = []Position434{*p.Position, *p.Position}
	a := &ObjectiveAttempt434{Candidate: ObjectiveCandidate434{GUID: n.GUID, Entry: 1234, Health: 42, MaxHealth: 42}, Observation: QuestProgressObservation434{World: nav.World, Quest: q}}
	approach := &ObjectiveAttempt434{Navigation: *nav}
	c := &objectiveCombatController434{result: a, approach: approach, factions: f}
	c.query = questProgressController434{result: &a.Observation, done: true}
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

func TestObjective434OneCreditAndNoSecondAttack(t *testing.T) {
	c, p, n := combatFixture434(t)
	var ops []uint16
	send := func(op uint16, b []byte) error { ops = append(ops, op); return nil }
	if err := c.tick(1000, send); err != nil {
		t.Fatal(err)
	}
	if !c.result.CommandedAttack || c.result.ObservedAttack {
		t.Fatal("command conflated with observation")
	}
	start := binary.LittleEndian.AppendUint64(nil, p.GUID)
	start = binary.LittleEndian.AppendUint64(start, n.GUID)
	if err := c.observe(loginPacket434{op: 0x2d15, body: start}); err != nil {
		t.Fatal(err)
	}
	n.Fields[FieldHealth434] = 20
	if err := c.tick(1500, send); err != nil {
		t.Fatal(err)
	}
	n.Fields[FieldHealth434] = 0
	if err := c.tick(2000, send); err != nil {
		t.Fatal(err)
	}
	if c.result.Passed || !c.result.Stopped {
		t.Fatal("death alone passed or did not stop")
	}
	p.Fields[questLogBase434+2] = 1
	c.result.Observation.Events = []QuestProgressEvent434{{Opcode: cataQuestCredit, QuestID: 900, Target: 1234, Count: 1, Required: 6, VictimGUID: n.GUID}}
	c.result.ObservedAttack = false
	if err := c.tick(3600, send); err != nil {
		t.Fatal(err)
	}
	if !c.result.Passed {
		t.Fatal("missing pass")
	}
	for i := 0; i < 10; i++ {
		if err := c.tick(3700, send); err != nil {
			t.Fatal(err)
		}
	}
	counts := map[uint16]int{}
	for _, op := range ops {
		counts[op]++
		switch op {
		case cataSetActiveMover, cataMoveHeartbeat, cataSetSelection, cataAttackSwing434, cataAttackStop434:
		default:
			t.Fatalf("unexpected opcode %04X", op)
		}
	}
	if counts[cataAttackSwing434] != 1 || counts[cataAttackStop434] != 1 {
		t.Fatal(counts)
	}
}

func TestObjective434AbortProtection(t *testing.T) {
	for _, name := range []string{"nonzero start", "more than one", "wrong target", "danger", "death", "invalid target", "rejected", "no start", "timeout", "wrong credit", "credit without death", "pet"} {
		t.Run(name, func(t *testing.T) {
			c, p, n := combatFixture434(t)
			var ops []uint16
			send := func(op uint16, b []byte) error { ops = append(ops, op); return nil }
			if name == "nonzero start" {
				p.Fields[questLogBase434+2] = 1
			} else if name == "pet" {
				p.Fields[0xa] = 9
			} else {
				c.result.CommandedAttack = true
				c.result.ObservedAttack = true
				c.startedAt = 1000
			}
			now := uint32(2000)
			switch name {
			case "more than one":
				p.Fields[questLogBase434+2] = 2
			case "wrong target":
				p.Fields[combatTarget434] = 88
			case "danger":
				p.Fields[FieldHealth434] = 60
			case "death":
				p.Fields[FieldHealth434] = 0
			case "invalid target":
				n.Fields[FieldEntry434]++
			case "rejected":
				c.pending = fmt.Errorf("COMBAT_STATE_BUG: rejection")
			case "no start":
				c.result.ObservedAttack = false
				now = 5000
			case "timeout":
				now = 47000
			case "wrong credit":
				c.result.Observation.Events = []QuestProgressEvent434{{Opcode: cataQuestCredit, QuestID: 900, Target: 1234, Count: 1, Required: 6, VictimGUID: n.GUID + 1}}
			case "credit without death":
				p.Fields[questLogBase434+2] = 1
				c.finishAt = 1000
				now = 7000
			}
			if err := c.tick(now, send); err == nil {
				t.Fatal("unsafe case accepted")
			}
			if c.result.Passed {
				t.Fatal("false pass")
			}
			if name == "nonzero start" || name == "pet" {
				if len(ops) != 0 {
					t.Fatal("sent before safe preconditions", ops)
				}
			} else if len(ops) < 1 || ops[0] != cataAttackStop434 {
				t.Fatal("missing abort stop", ops)
			}
		})
	}
}

func TestObjective434RangesAndRequirements(t *testing.T) {
	c, p, n := combatFixture434(t)
	if r, e := meleeRange434(p, n); e != nil || r != 5 {
		t.Fatal(r, e)
	}
	p.Fields[combatReach434] = math.Float32bits(3)
	n.Fields[combatReach434] = math.Float32bits(2)
	if r, e := meleeRange434(p, n); e != nil || math.Abs(r-6.3333334) > 0.00001 {
		t.Fatal(r, e)
	}
	p.Fields[combatReach434] = math.Float32bits(float32(math.NaN()))
	if _, e := meleeRange434(p, n); e == nil {
		t.Fatal("NaN accepted")
	}
	for _, count := range []uint32{0, 1, 2, 6} {
		q := c.result.Observation.Quest
		q.Objectives = append([]QuestObjectiveProgress434(nil), q.Objectives...)
		q.Objectives[0].Current = count
		if o, e := singleObjective434(q, 0); (e == nil) != (count == 0) || o.Target != 1234 {
			t.Fatal(o, e)
		}
		if _, e := singleObjective434(q, 1); (e == nil) != (count == 1) {
			t.Fatal(count, e)
		}
	}
}
