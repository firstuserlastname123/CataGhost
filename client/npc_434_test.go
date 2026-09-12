package client

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func gossipFixture434(options, quests bool) []byte {
	b := targetGUID434(0xf130000123000456)
	u := func(n uint32) { b = binary.LittleEndian.AppendUint32(b, n) }
	u(77)
	u(88)
	if options {
		u(1)
		u(9)
		b = append(b, 2, 0)
		u(0)
		b = append(b, []byte("Hello\x00Confirm\x00")...)
	} else {
		u(0)
	}
	if quests {
		u(1)
		u(111)
		u(2)
		u(3)
		u(4)
		b = append(b, 1)
		b = append(b, []byte("Synthetic quest\x00")...)
	} else {
		u(0)
	}
	return b
}
func TestNPC434GUIDAndGossip(t *testing.T) {
	if cataSetSelection != 0x0506 || cataGossipHello != 0x4525 || cataGossipMessage != 0x2035 || cataGossipComplete != 0x0806 {
		t.Fatal("opcode")
	}
	if !bytes.Equal(targetGUID434(0x0102030405060708), []byte{8, 7, 6, 5, 4, 3, 2, 1}) || !bytes.Equal(targetGUID434(0), make([]byte, 8)) {
		t.Fatal("GUID layout")
	}
	for _, options := range []bool{false, true} {
		for _, quests := range []bool{false, true} {
			b := gossipFixture434(options, quests)
			g, e := parseGossip434(b, 0xf130000123000456)
			if e != nil || g.Menu != 77 || g.Text != 88 {
				t.Fatal(g, e)
			}
			if (len(g.Options) > 0) != options || (len(g.Quests) > 0) != quests {
				t.Fatal(g)
			}
			if quests && (!g.Quests[0].Repeatable || g.Quests[0].Title != "Synthetic quest") {
				t.Fatal(g)
			}
			for i := 0; i < len(b); i++ {
				if _, e := parseGossip434(b[:i], 0xf130000123000456); e == nil {
					t.Fatalf("truncation %d accepted", i)
				}
			}
			if _, e := parseGossip434(b, 7); e == nil {
				t.Fatal("wrong GUID")
			}
			if _, e := parseGossip434(append(b, 0), 0xf130000123000456); e == nil {
				t.Fatal("trailing bytes")
			}
		}
	}
	b := gossipFixture434(false, false)
	binary.LittleEndian.PutUint32(b[16:], 0xffffffff)
	if _, e := parseGossip434(b, 0xf130000123000456); e == nil {
		t.Fatal("malformed count")
	}
}
func npcFixture434() (*ObjectStore434, *NPCFactions434, []reputation434) {
	s := &ObjectStore434{PlayerGUID: 17, objects: map[uint64]*Object434{}}
	s.objects[17] = &Object434{GUID: 17, Type: 4, Map: 1, Position: &Position434{}, Fields: map[uint16]uint32{FieldFaction434: 1, FieldUnitBytes434: 4 | 3<<8}}
	g := uint64(0xf130000123000456)
	s.objects[g] = &Object434{GUID: g, Type: 3, Created: true, Map: 1, Position: &Position434{X: 10}, Fields: map[uint16]uint32{FieldFaction434: 2, FieldHealth434: 50, FieldNPCFlags434: 3, FieldEntry434: 291}}
	a, b := make([]uint32, 14), make([]uint32, 14)
	a[0] = 1
	b[0] = 2
	b[1] = 100
	faction := make([]uint32, 18)
	faction[0] = 100
	faction[1] = 0
	faction[2] = 1 << 3
	faction[10] = 3000
	return s, &NPCFactions434{templates: map[uint32][]uint32{1: a, 2: b}, factions: map[uint32][]uint32{100: faction}}, make([]reputation434, 256)
}
func TestNPC434DiscoveryAndApproach(t *testing.T) {
	s, f, r := npcFixture434()
	c := npcCandidates434(s, f, r)
	if len(c) != 1 || c[0].Distance != 10 {
		t.Fatal(c)
	}
	d, e := npcDestination434(*s.objects[17].Position, c[0])
	if e != nil || d.X != 7.5 || d.Y != 0 {
		t.Fatal(d, e)
	}
	if _, e := npcDestination434(Position434{X: 10}, c[0]); e == nil {
		t.Fatal("coincident approach")
	}
	if _, e := npcDestination434(Position434{X: float32(math.NaN())}, c[0]); e == nil {
		t.Fatal("NaN")
	}
	other := *s.objects[c[0].GUID]
	other.GUID++
	s.objects[other.GUID] = &other
	list := npcCandidates434(s, f, r)
	if len(list) != 2 || list[0].GUID != c[0].GUID {
		t.Fatal("unstable order")
	}
	for _, name := range []string{"dead", "not gossip", "combat", "pet", "player", "charmed", "hostile", "missing reputation", "at war", "distant"} {
		t.Run(name, func(t *testing.T) {
			s, f, r := npcFixture434()
			o := s.objects[c[0].GUID]
			switch name {
			case "dead":
				o.Fields[FieldHealth434] = 0
			case "not gossip":
				o.Fields[FieldNPCFlags434] = 2
			case "combat":
				o.Fields[0x35] = 0x80000
			case "pet":
				o.GUID = 0xf140000123000456
			case "player":
				o.Type = 4
			case "charmed":
				o.Fields[0xe] = 1
			case "hostile":
				r[0].standing = -10000
			case "missing reputation":
				r = nil
			case "at war":
				r[0].flags = 2
			case "distant":
				o.Position.X = 100
			}
			if len(npcCandidates434(s, f, r)) != 0 {
				t.Fatal("unsafe candidate accepted")
			}
		})
	}
}
func TestNPC434ReputationBounds(t *testing.T) {
	b := binary.LittleEndian.AppendUint32(nil, 256)
	b = append(b, make([]byte, 256*5)...)
	if r, e := parseReputations434(b); e != nil || len(r) != 256 {
		t.Fatal(e)
	}
	for i := 0; i < len(b); i++ {
		if _, e := parseReputations434(b[:i]); e == nil {
			t.Fatal(i)
		}
	}
}
func TestNPC434FactionDBC(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f.dbc")
	b := []byte("WDBC")
	for _, n := range []uint32{1, 14, 56, 1} {
		b = binary.LittleEndian.AppendUint32(b, n)
	}
	b = append(b, make([]byte, 57)...)
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := readFactionDBC434(p, 14); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, b[:len(b)-1], 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := readFactionDBC434(p, 14); e == nil {
		t.Fatal("short DBC accepted")
	}
}
