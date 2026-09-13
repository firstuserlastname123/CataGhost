package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"github.com/azerothcore/AzerothGhost/pathfinding"
	"testing"
)

// Independent writer follows TCPP QuestPackets.cpp field order. Deliberately
// fills fixed reward arrays even when reported counts are smaller.
func questDetailsFixture434() []byte {
	b := new(bytes.Buffer)
	put := func(v any) {
		if err := binary.Write(b, binary.LittleEndian, v); err != nil {
			panic(err)
		}
	}
	put(uint64(0xf130000001000002))
	put(uint64(0))
	put(uint32(321))
	for _, s := range []string{"Synthetic quest", "Description", "Objectives", "giver text", "giver name", "turn-in text", "turn-in name"} {
		b.WriteString(s)
		b.WriteByte(0)
	}
	put(uint32(7))
	put(uint32(8))
	put(uint8(1))
	put(uint32(8))
	put(uint32(0))
	put(uint8(0))
	put(uint8(0))
	put(uint32(0))
	put(uint32(1))
	for i := 0; i < 18; i++ {
		put(uint32(i + 1))
	}
	put(uint32(2))
	for i := 0; i < 12; i++ {
		put(uint32(i + 101))
	}
	for i := 0; i < 8; i++ {
		put(uint32(i + 201))
	}
	for i := 0; i < 15; i++ {
		put(uint32(i + 301))
	}
	put(uint32(401))
	put(uint32(402))
	for i := 0; i < 8; i++ {
		put(uint32(i + 501))
	}
	put(uint32(601))
	put(uint32(602))
	put(uint32(4))
	for i := 0; i < 8; i++ {
		put(uint32(i + 701))
	}
	return b.Bytes()
}

func TestQuest434SafeApproachCandidates(t *testing.T) {
	p := Position434{X: 10, Y: 10, Z: 5}
	n := NPCCandidate434{Position: Position434{X: 12, Y: 10, Z: 5}}
	count := 0
	f := routeFunc434(func(m uint32, s, d pathfinding.Point3D) (*pathfinding.PathResult, error) {
		count++
		if m != 1 || distance434(d, point434(n.Position)) > 2.501 {
			t.Fatal("candidate geometry")
		}
		r := navPath434(s, d)
		if count == 1 {
			r.Type = pathfinding.PathfindIncomplete
		}
		if count == 2 {
			r.Points[1].Z += 5
		}
		return r, nil
	})
	if _, err := safeQuestApproach434(context.Background(), f, 1, p, n); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	bad := routeFunc434(func(_ uint32, s, d pathfinding.Point3D) (*pathfinding.PathResult, error) {
		return nil, errors.New("unreachable")
	})
	if _, err := safeQuestApproach434(context.Background(), bad, 1, p, n); err == nil {
		t.Fatal("no safe route accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := safeQuestApproach434(ctx, f, 1, p, n); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	// Exact rejected authoritative-to-mesh segment, retained without changing the guard.
	s := pathfinding.Point3D{X: 10310.29, Y: 830.7877, Z: 1326.6477}
	d := pathfinding.Point3D{X: 10310.379, Y: 829.19354, Z: 1326.53}
	r := &pathfinding.PathResult{Type: pathfinding.PathfindNormal, Points: []pathfinding.Point3D{{X: s.X, Y: s.Y, Z: 1327.2178}, {X: d.X, Y: d.Y, Z: 1327.1288}}}
	if _, err := validateBoundedRoute434(s, d, r, 40, 1, 50, 64); err == nil {
		t.Fatal("slope safeguard weakened")
	}
}
func TestQuest434WireAndDetails(t *testing.T) {
	g := uint64(0xf130000001000002)
	if cataQuestDetailsRequest != 0x2f14 || cataQuestDetails != 0x2425 || cataQuestAccept != 0x6b37 {
		t.Fatal("opcodes")
	}
	if got := questDetailsRequest434(0x0102030405060708, 0xa1b2c3d4); !bytes.Equal(got, []byte{8, 7, 6, 5, 4, 3, 2, 1, 0xd4, 0xc3, 0xb2, 0xa1, 0}) {
		t.Fatalf("query %x", got)
	}
	if got := questAccept434(0x0102030405060708, 0xa1b2c3d4); !bytes.Equal(got, []byte{8, 7, 6, 5, 4, 3, 2, 1, 0xd4, 0xc3, 0xb2, 0xa1, 0, 0, 0, 0}) {
		t.Fatalf("accept %x", got)
	}
	b := questDetailsFixture434()
	d, e := parseQuestDetails434(b, g, 321)
	if e != nil || d.ID != 321 || d.Title != "Synthetic quest" || d.Rewards.Choices[0] != (QuestItem434{1, 7, 13}) || d.Rewards.Items[0] != (QuestItem434{101, 105, 109}) || d.Rewards.Money != 201 || d.Rewards.XP != 202 || d.Rewards.SkillUps != 602 || len(d.Emotes) != 4 {
		t.Fatal(d, e)
	}
	for i := 0; i < len(b); i++ {
		if _, e := parseQuestDetails434(b[:i], g, 321); e == nil {
			t.Fatalf("truncation %d", i)
		}
	}
	if _, e := parseQuestDetails434(b, g+1, 321); e == nil {
		t.Fatal("giver mismatch")
	}
	if _, e := parseQuestDetails434(b, g, 322); e == nil {
		t.Fatal("quest mismatch")
	}
	if _, e := parseQuestDetails434(append(b, 0), g, 321); e == nil {
		t.Fatal("trailing bytes")
	}
	// Locate choice count following the seven terminated strings and metadata.
	offset := 20
	for i := 0; i < 7; i++ {
		offset += bytes.IndexByte(b[offset:], 0) + 1
	}
	offset += 23
	for _, change := range []struct {
		offset int
		value  uint32
	}{{offset, 7}, {offset + 76, 5}, {len(b) - 36, 5}} {
		bad := append([]byte(nil), b...)
		binary.LittleEndian.PutUint32(bad[change.offset:], change.value)
		if _, e := parseQuestDetails434(bad, g, 321); e == nil {
			t.Fatal("count accepted", change)
		}
	}
}
func TestQuest434LogAndSelection(t *testing.T) {
	a := navFixture434(t)
	p := a.World.Store.objects[17]
	log, e := QuestLog434(&a.World.Store)
	if e != nil || len(log) != 0 {
		t.Fatal(log, e)
	}
	p.Fields[questLogBase434] = 55
	p.Fields[questLogBase434+1] = 1
	p.Fields[questLogBase434+2] = 2 | 3<<16
	p.Fields[questLogBase434+3] = 4 | 5<<16
	p.Fields[questLogBase434+4] = 123
	log, e = QuestLog434(&a.World.Store)
	if e != nil || len(log) != 1 || log[0].Counters != [4]uint16{2, 3, 4, 5} || log[0].Timer != 123 {
		t.Fatal(log, e)
	}
	g := Gossip434{Quests: []GossipQuest434{{ID: 55, Type: 2, Level: 1}, {ID: 88, Type: 4, Level: 1}, {ID: 100, Type: 2, Level: 1, Flags: 0x80000}, {ID: 66, Type: 2, Level: 1, Flags: 8}, {ID: 77, Type: 2, Level: 1}}}
	q, e := selectQuest434(g, log, 1)
	if e != nil || q.ID != 66 {
		t.Fatal(q, e)
	}
	if _, e := selectQuest434(Gossip434{}, log, 1); e == nil {
		t.Fatal("no available")
	}
	p.Fields[questLogBase434+5] = 55
	if _, e := QuestLog434(&a.World.Store); e == nil {
		t.Fatal("duplicate log")
	}
}
func TestQuest434ConversationProofAndFailures(t *testing.T) {
	for _, mode := range []string{"accept", "preview", "timeout", "reject", "send-fail", "already", "wrong-inspected"} {
		t.Run(mode, func(t *testing.T) {
			a := NPCInteraction434{World: navFixture434(t).World, NPC: NPCCandidate434{GUID: 0xf130000001000002}, Gossip: Gossip434{Quests: []GossipQuest434{{ID: 321, Type: 2, Level: 1, Flags: 8}}}}
			r := QuestAcceptance434{}
			c := questConversation434{result: &r, approvedID: 321}
			if mode == "preview" {
				c.approvedID = 0
			}
			if mode == "wrong-inspected" {
				c.approvedID = 999
			}
			if mode == "already" {
				a.World.Store.objects[17].Fields[questLogBase434] = 321
			}
			var ops []uint16
			send := func(op uint16, b []byte) error {
				if op != cataQuestDetailsRequest && op != cataQuestAccept {
					t.Fatal("unrelated gameplay", op)
				}
				ops = append(ops, op)
				if mode == "send-fail" {
					return errors.New("send failed")
				}
				return nil
			}
			_, e := c.tick(100, send, &a)
			if mode == "send-fail" || mode == "already" || mode == "wrong-inspected" {
				if e == nil || r.AcceptSent {
					t.Fatal(e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if mode == "timeout" {
				if _, e := c.tick(8200, send, &a); e == nil {
					t.Fatal("timeout")
				}
				return
			}
			if mode == "reject" {
				if e := c.observe(loginPacket434{op: cataQuestInvalid, body: []byte{6, 0, 0, 0}}, &a); e == nil {
					t.Fatal("rejection")
				}
				return
			}
			if e := c.observe(loginPacket434{op: cataQuestDetails, body: questDetailsFixture434()}, &a); e != nil {
				t.Fatal(e)
			}
			done, e := c.tick(200, send, &a)
			if e != nil {
				t.Fatal(e)
			}
			if mode == "preview" {
				if !done || r.AcceptSent || len(ops) != 1 {
					t.Fatal("preview accepted")
				}
				return
			}
			if done || !r.AcceptSent || r.Accepted != nil {
				t.Fatal("commanded state became accepted")
			}
			for i := 0; i < 3; i++ {
				if done, e := c.tick(300, send, &a); e != nil || done {
					t.Fatal(e)
				}
			}
			if len(ops) != 2 {
				t.Fatal("multiple accepts")
			}
			// Ordinary values-only UPDATE_OBJECT, not local intent, adds the quest.
			b := []byte{1, 0, 1, 0, 0, 0, 0, 1, 17, 5}
			for i := 0; i < 4; i++ {
				b = binary.LittleEndian.AppendUint32(b, 0)
			}
			b = binary.LittleEndian.AppendUint32(b, 1<<29)
			b = binary.LittleEndian.AppendUint32(b, 321)
			if e := a.World.Store.ApplyUpdate(b); e != nil {
				t.Fatal(e)
			}
			done, e = c.tick(400, send, &a)
			if e != nil || !done || r.Accepted == nil {
				t.Fatal(e)
			}
			r.Interaction = a
			fresh := navFixture434(t).World
			fresh.Store.objects[17].Fields[questLogBase434] = 321
			if _, e := VerifyQuestReconnect434(r, fresh); e != nil {
				t.Fatal(e)
			}
			delete(fresh.Store.objects[17].Fields, questLogBase434)
			if _, e := VerifyQuestReconnect434(r, fresh); e == nil {
				t.Fatal("absent reconnect quest accepted")
			}
		})
	}
}

func TestQuest434AutoFlagExplicitAccept(t *testing.T) {
	a := NPCInteraction434{World: navFixture434(t).World, NPC: NPCCandidate434{GUID: 0xf130000001000002}, Gossip: Gossip434{Quests: []GossipQuest434{{ID: 987, Type: 2, Level: 1, Flags: 0x80000}}}}
	r := QuestAcceptance434{}
	c := questConversation434{result: &r}
	calls := 0
	send := func(op uint16, body []byte) error {
		calls++
		if op != cataQuestAccept || !bytes.Equal(body, questAccept434(a.NPC.GUID, 987)) {
			t.Fatalf("unexpected query/gameplay packet: %04x %x", op, body)
		}
		return nil
	}
	if done, err := c.tick(100, send, &a); err != nil || done || !r.AcceptSent || r.Accepted != nil {
		t.Fatalf("send is not proof: %+v %v", r, err)
	}
	if done, err := c.tick(200, send, &a); err != nil || done || calls != 1 {
		t.Fatal("must wait without repeating acceptance", err)
	}
	a.World.Store.objects[17].Fields[questLogBase434] = 987
	if done, err := c.tick(300, send, &a); err != nil || !done || r.Accepted == nil || calls != 1 {
		t.Fatal("server addition required", err)
	}
	r.Interaction = a
	fresh := navFixture434(t).World
	fresh.Store.objects[17].Fields[questLogBase434] = 987
	if _, err := VerifyQuestReconnect434(r, fresh); err != nil {
		t.Fatal(err)
	}
	a.Gossip.Quests[0].Flags |= 0x10000 // forbidden flag must not be hidden by AUTO_ACCEPT
	if _, err := selectQuest434(a.Gossip, nil, 1); err == nil {
		t.Fatal("unsafe flag combination accepted")
	}
}
