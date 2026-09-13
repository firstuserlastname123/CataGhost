package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

const (
	cataSetSelection              = 0x0506
	cataGossipHello               = 0x4525
	cataGossipMessage             = 0x2035
	cataGossipComplete            = 0x0806
	cataInitializeFactions        = 0x4634
	FieldEntry434          uint16 = 5
	FieldNPCFlags434       uint16 = 0x4d
)

// Selection and Hello both use ObjectGuid's ordinary uint64 wire operator.
func targetGUID434(g uint64) []byte { return binary.LittleEndian.AppendUint64(nil, g) }

type GossipOption434 struct {
	ID            uint32
	Icon, Flags   uint8
	Cost          uint32
	Text, Confirm string
}
type GossipQuest434 struct {
	ID, Type, Level, Flags uint32
	Repeatable             bool
	Title                  string
}
type Gossip434 struct {
	GUID       uint64
	Menu, Text uint32
	Options    []GossipOption434
	Quests     []GossipQuest434
}

func gossipString434(r *objectReader434) string {
	if r.err != nil {
		return ""
	}
	rest := r.b[r.bit/8:]
	n := bytes.IndexByte(rest, 0)
	if n < 0 || n > 8192 {
		r.err = fmt.Errorf("invalid gossip string")
		return ""
	}
	return string(r.take(n + 1)[:n])
}
func parseGossip434(b []byte, expected uint64) (Gossip434, error) {
	r := newObjectReader434(b)
	g := Gossip434{GUID: r.u64(), Menu: r.u32(), Text: r.u32()}
	if expected == 0 || g.GUID != expected {
		return g, fmt.Errorf("gossip GUID mismatch")
	}
	n := int(r.u32())
	if n > 256 || !r.bounded(n, 12) {
		return g, fmt.Errorf("invalid gossip option count")
	}
	for i := 0; i < n; i++ {
		g.Options = append(g.Options, GossipOption434{r.u32(), r.u8(), r.u8(), r.u32(), gossipString434(r), gossipString434(r)})
	}
	n = int(r.u32())
	if n > 256 || !r.bounded(n, 18) {
		return g, fmt.Errorf("invalid gossip quest count")
	}
	for i := 0; i < n; i++ {
		q := GossipQuest434{ID: r.u32(), Type: r.u32(), Level: r.u32(), Flags: r.u32()}
		flag := r.u8()
		if flag > 1 {
			return g, fmt.Errorf("invalid repeatable flag")
		}
		q.Repeatable = flag == 1
		q.Title = gossipString434(r)
		g.Quests = append(g.Quests, q)
	}
	return g, r.done()
}

// Faction tables are read-only build-15595 DBC data, never SQL queries.
type NPCFactions434 struct{ templates, factions map[uint32][]uint32 }

func readFactionDBC434(path string, minimum int) (map[uint32][]uint32, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) < 20 || string(b[:4]) != "WDBC" {
		return nil, fmt.Errorf("invalid faction DBC header")
	}
	u := func(i int) uint32 { return binary.LittleEndian.Uint32(b[i : i+4]) }
	n, fields, stride, strings := uint64(u(4)), uint64(u(8)), uint64(u(12)), uint64(u(16))
	if n > 100000 || fields < uint64(minimum) || fields > 64 || stride != fields*4 || 20+n*stride+strings != uint64(len(b)) {
		return nil, fmt.Errorf("invalid faction DBC bounds")
	}
	rows := map[uint32][]uint32{}
	for i := uint64(0); i < n; i++ {
		row := make([]uint32, int(fields))
		for j := range row {
			row[j] = u(int(20 + i*stride + uint64(j)*4))
		}
		if _, ok := rows[row[0]]; ok {
			return nil, fmt.Errorf("duplicate faction row")
		}
		rows[row[0]] = row
	}
	return rows, nil
}
func LoadNPCFactions434(dir string) (*NPCFactions434, error) {
	t, e := readFactionDBC434(filepath.Join(dir, "FactionTemplate.dbc"), 14)
	if e != nil {
		return nil, e
	}
	f, e := readFactionDBC434(filepath.Join(dir, "Faction.dbc"), 18)
	if e != nil {
		return nil, e
	}
	return &NPCFactions434{t, f}, nil
}

type reputation434 struct {
	flags    byte
	standing int32
}

func parseReputations434(b []byte) ([]reputation434, error) {
	r := newObjectReader434(b)
	n := r.u32()
	if n != 256 {
		return nil, fmt.Errorf("expected 256 initial factions")
	}
	out := make([]reputation434, n)
	for i := range out {
		out[i] = reputation434{r.u8(), int32(r.u32())}
	}
	return out, r.done()
}
func templateFriendly434(a, b []uint32) bool {
	if a[0] == b[0] {
		return true
	}
	for _, v := range a[10:14] {
		if v != 0 && v == b[1] {
			return true
		}
	}
	return a[3]&b[4] != 0
}
func (f *NPCFactions434) friendly(npc, player Object434, rep []reputation434) bool {
	a, b := f.templates[npc.Fields[FieldFaction434]], f.templates[player.Fields[FieldFaction434]]
	if len(a) < 14 || len(b) < 14 || a[2]&0x1000 != 0 {
		return false
	}
	faction := f.factions[a[1]]
	if len(faction) < 18 {
		return false
	}
	index := int32(faction[1])
	if index >= 0 && player.Fields[0x36]&4 == 0 {
		if int(index) >= len(rep) || rep[index].flags&2 != 0 {
			return false
		}
		packed := player.Fields[FieldUnitBytes434]
		race, class := packed&255, (packed>>8)&255
		if race == 0 || race > 32 || class == 0 || class > 32 {
			return false
		}
		rm, cm := uint32(1)<<(race-1), uint32(1)<<(class-1)
		base := int32(0)
		for i := 0; i < 4; i++ {
			if (faction[2+i]&rm != 0 || faction[2+i] == 0 && faction[6+i] != 0) && (faction[6+i]&cm != 0 || faction[6+i] == 0 && faction[10+i] != 0) {
				base = int32(faction[10+i])
				break
			}
		}
		return int64(base)+int64(rep[index].standing) >= 3000 // friendly, not merely neutral
	}
	if a[0] != b[0] {
		if a[2]&0x2000 != 0 && !templateFriendly434(a, b) {
			return false
		}
		for _, v := range a[6:10] {
			if v != 0 && v == b[1] {
				return false
			}
		}
		if a[3]&b[5] != 0 {
			return false
		}
	}
	return templateFriendly434(a, b) || templateFriendly434(b, a)
}

type NPCCandidate434 struct {
	GUID                          uint64
	Entry, Flags, Health, Faction uint32
	Position                      Position434
	Distance                      float64
}

func npcCandidates434(s *ObjectStore434, f *NPCFactions434, rep []reputation434) []NPCCandidate434 {
	return npcCandidatesWithin434(s, f, rep, 4)
}
func npcCandidatesWithin434(s *ObjectStore434, f *NPCFactions434, rep []reputation434, minDistance float64) []NPCCandidate434 {
	p := s.objects[s.PlayerGUID]
	if p == nil || p.Position == nil || f == nil {
		return nil
	}
	var out []NPCCandidate434
	for _, o := range s.Objects() {
		if o.Type != 3 || !o.Created || o.GUID>>52 != 0xf13 || o.Position == nil || !finitePosition434(o.Position) || o.Map != p.Map || o.Fields[FieldHealth434] == 0 || o.Fields[FieldNPCFlags434]&1 == 0 || o.Fields[0x35]&(0x80000|0x2000000) != 0 || o.Fields[0xe] != 0 || o.Fields[0xf] != 0 || o.Fields[0x10] != 0 || o.Fields[0x11] != 0 {
			continue
		}
		if o.Movement != nil && o.Movement.Moving() {
			continue
		}
		if !f.friendly(o, *p, rep) {
			continue
		}
		d := distance434(point434(*p.Position), point434(*o.Position))
		if d < minDistance || d > 40 {
			continue
		}
		out = append(out, NPCCandidate434{o.GUID, o.Fields[FieldEntry434], o.Fields[FieldNPCFlags434], o.Fields[FieldHealth434], o.Fields[FieldFaction434], *o.Position, d})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Distance != out[j].Distance {
			return out[i].Distance < out[j].Distance
		}
		return out[i].GUID < out[j].GUID
	})
	return out
}
func npcDestination434(p Position434, n NPCCandidate434) (Position434, error) {
	dx, dy := float64(p.X-n.Position.X), float64(p.Y-n.Position.Y)
	d := math.Hypot(dx, dy)
	if d < 3 || d > 40 || !finitePosition434(&p) || !finitePosition434(&n.Position) {
		return Position434{}, fmt.Errorf("invalid NPC approach distance")
	}
	return Position434{X: n.Position.X + float32(2.5*dx/d), Y: n.Position.Y + float32(2.5*dy/d), Z: n.Position.Z, Orientation: float32(math.Atan2(-dy, -dx))}, nil
}
