package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
)

// Independent TCPP QueryQuestInfoResponse writer: fixed metadata/rewards,
// five strings, four NPC/GO tuples, six item tuples, spell/text/currency/portraits.
func questDefinitionFixture434() []byte {
	b := new(bytes.Buffer)
	put := func(v uint32) { _ = binary.Write(b, binary.LittleEndian, v) }
	str := func(s string) { b.WriteString(s); b.WriteByte(0) }
	header := []uint32{900, 2, 12, 10, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	for _, v := range header {
		put(v)
	}
	for i := 0; i < 4; i++ {
		put(uint32(100 + i))
		put(1)
	} // guaranteed rewards
	for i := 0; i < 6; i++ {
		put(uint32(200 + i))
		put(2)
	} // choices
	for i := 0; i < 15; i++ {
		put(uint32(300 + i))
	} // faction arrays
	put(1)
	put(0x3f800000)
	put(0x40000000)
	put(0)
	for _, s := range []string{"Synthetic objectives", "Credit and collect", "Description", "Area", "Return text"} {
		str(s)
	}
	for _, tuple := range [][4]uint32{{1234, 6, 0, 0}, {0x80000567, 2, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}} {
		for _, v := range tuple {
			put(v)
		}
	}
	for i := 0; i < 6; i++ {
		if i == 0 {
			put(789)
			put(3)
		} else {
			put(0)
			put(0)
		}
	}
	put(0)
	for _, s := range []string{"Creature credit", "Use object", "", ""} {
		str(s)
	}
	for i := 0; i < 8; i++ {
		put(0)
		put(0)
	} // reward then required currencies
	for i := 0; i < 4; i++ {
		str("")
	}
	put(0)
	put(0)
	return b.Bytes()
}

func TestQuestProgress434Definition(t *testing.T) {
	b := questDefinitionFixture434()
	d, err := parseQuestDefinition434(b)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != 900 || d.Title != "Synthetic objectives" || d.Level != 12 || d.Targets[0][0] != 1234 || d.Items[0][0] != 789 || d.Prefix[31] != 100 || d.Prefix[66] != 1 {
		t.Fatal(d)
	}
	qs := d.Requirements()
	if len(qs) != 3 || qs[1].Kind != "gameobject-credit" || qs[1].Target != 0x567 || qs[2].Kind != "item" {
		t.Fatal(qs)
	}
	for i := 0; i < len(b); i++ {
		if _, err := parseQuestDefinition434(b[:i]); err == nil {
			t.Fatalf("truncated %d", i)
		}
	}
	if _, err := parseQuestDefinition434(append(b, 0)); err == nil {
		t.Fatal("trailing bytes")
	}
	bad := append([]byte(nil), b...)
	binary.LittleEndian.PutUint32(bad, 0)
	if _, err := parseQuestDefinition434(bad); err == nil {
		t.Fatal("zero ID")
	}
	if !bytes.Equal(questInfoRequest434(0x12345678), []byte{0x78, 0x56, 0x34, 0x12}) || cataQuestQueryInfo != 0xd06 || cataQuestQueryResponse != 0x6936 {
		t.Fatal("query wire")
	}
	d.Type = 999
	d.Flags = 0x80000000
	d.Targets = [4][4]uint32{}
	d.Items = [6][2]uint32{}
	if d.Requirements()[0].Kind != "unknown" || d.Type != 999 {
		t.Fatal("unknown semantics lost")
	}
}

func TestQuestProgress434StatesInventoryAndContinuity(t *testing.T) {
	d, _ := parseQuestDefinition434(questDefinitionFixture434())
	s := navFixture434(t).World.Store
	p := s.objects[17]
	if _, err := ProjectQuestProgress434(d, &s); err == nil {
		t.Fatal("absent quest")
	}
	p.Fields[questLogBase434] = 900
	for _, tc := range []struct {
		state uint32
		want  string
	}{{0, "incomplete"}, {1, "complete"}, {2, "failed"}, {3, "unknown"}, {8, "unknown"}} {
		p.Fields[questLogBase434+1] = tc.state
		got, err := ProjectQuestProgress434(d, &s)
		if err != nil || got.Status != tc.want || got.Objectives[0].Current != 0 || got.Objectives[0].Complete {
			t.Fatal(got, err)
		}
	}
	p.Fields[questLogBase434+1] = 0
	p.Fields[questLogBase434+4] = 1900000000
	// Values-only packet replaces both uint16 counter halves: creature 3, GO 2.
	update := []byte{1, 0, 1, 0, 0, 0, 0, 1, 17, 5}
	for i := 0; i < 4; i++ {
		update = binary.LittleEndian.AppendUint32(update, 0)
	}
	update = binary.LittleEndian.AppendUint32(update, 1<<31)
	update = binary.LittleEndian.AppendUint32(update, 3|2<<16)
	if err := s.ApplyUpdate(update); err != nil {
		t.Fatal(err)
	}
	got, err := ProjectQuestProgress434(d, &s)
	if err != nil || got.Objectives[0].Current != 3 || got.Objectives[0].Complete || !got.Objectives[1].Complete || got.Slot.Timer != 1900000000 {
		t.Fatal(got, err)
	}
	binary.LittleEndian.PutUint32(update[len(update)-4:], 6|2<<16)
	if err := s.ApplyUpdate(update); err != nil {
		t.Fatal(err)
	}
	got, _ = ProjectQuestProgress434(d, &s)
	if !got.Objectives[0].Complete || got.Status != "incomplete" {
		t.Fatal("counts must not invent overall completion", got)
	}
	p.Fields[0x1c0] = 77
	got, _ = ProjectQuestProgress434(d, &s)
	if got.Objectives[2].Known {
		t.Fatal("missing item treated as zero")
	}
	s.objects[77] = &Object434{GUID: 77, Created: true, Type: 1, Fields: map[uint16]uint32{8: 17, FieldEntry434: 789, 0x10: 2}}
	got, _ = ProjectQuestProgress434(d, &s)
	if !got.Objectives[2].Known || got.Objectives[2].Current != 2 || got.Objectives[2].Complete {
		t.Fatal(got)
	}
	s.objects[77].Fields[0x10] = 3
	got, _ = ProjectQuestProgress434(d, &s)
	if !got.Objectives[2].Complete {
		t.Fatal(got)
	}
	s.objects[77].Fields[0x10] = 1
	got, _ = ProjectQuestProgress434(d, &s)
	if got.Objectives[2].Complete {
		t.Fatal("item removal lost")
	}
	d.Flags = 2
	got, _ = ProjectQuestProgress434(d, &s)
	if got.Objectives[len(got.Objectives)-1].Known {
		t.Fatal("incomplete does not mean unexplored")
	}
}

func TestQuestProgress434Notifications(t *testing.T) {
	for _, tc := range []struct {
		op    uint16
		words []uint32
		guid  bool
	}{{0xd27, []uint32{900, 0x80000567, 1, 2}, true}, {0x4416, []uint32{900, 2, 10}, false}, {0x2937, []uint32{900}, false}, {0x6427, []uint32{900}, false}, {0x4236, []uint32{900, 4}, false}} {
		var b []byte
		for _, w := range tc.words {
			b = binary.LittleEndian.AppendUint32(b, w)
		}
		if tc.guid {
			b = binary.LittleEndian.AppendUint64(b, 0xf110000002000003)
		}
		e, known, err := parseQuestProgressEvent434(tc.op, b)
		if err != nil || !known || e.QuestID != 900 {
			t.Fatal(e, err)
		}
		for i := 0; i < len(b); i++ {
			if _, _, err := parseQuestProgressEvent434(tc.op, b[:i]); err == nil {
				t.Fatal("truncated progress", tc.op, i)
			}
		}
		if _, _, err := parseQuestProgressEvent434(tc.op, append(b, 0)); err == nil {
			t.Fatal("trailing progress")
		}
	}
	if _, known, err := parseQuestProgressEvent434(0x6324, []byte{1}); known || err != nil {
		t.Fatal("guessed unused opcode")
	}
	o := QuestProgressObservation434{World: navFixture434(t).World}
	o.World.Store.objects[17].Fields[questLogBase434] = 900
	before := o.World.Store.Objects()
	c := questProgressController434{result: &o}
	if err := c.observe(loginPacket434{op: cataQuestProgressComplete, body: questInfoRequest434(900)}); err != nil {
		t.Fatal(err)
	}
	if len(o.Events) != 1 || !reflect.DeepEqual(before, o.World.Store.Objects()) {
		t.Fatal("notification changed authoritative fields")
	}
}

func TestQuestProgress434SessionController(t *testing.T) {
	o := QuestProgressObservation434{World: navFixture434(t).World}
	o.World.Store.objects[17].Fields[questLogBase434] = 900
	c := questProgressController434{result: &o}
	calls := 0
	send := func(op uint16, b []byte) error {
		calls++
		if op != 0xd06 || !bytes.Equal(b, questInfoRequest434(900)) {
			t.Fatal("unrelated action", op)
		}
		return nil
	}
	if err := c.tick(100, send); err != nil {
		t.Fatal(err)
	}
	if err := c.tick(1200, send); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || c.done {
		t.Fatal("query alone is not success")
	}
	if err := c.observe(loginPacket434{op: 0x6936, body: questDefinitionFixture434()}); err != nil {
		t.Fatal(err)
	}
	if err := c.tick(3000, send); err != nil || !c.done || calls != 1 {
		t.Fatal(err)
	}
	if err := VerifyQuestProgressReconnect434(o, o); err != nil {
		t.Fatal(err)
	}
	changed := o
	changed.Quest.Slot.Counters[0]++
	if err := VerifyQuestProgressReconnect434(o, changed); err == nil {
		t.Fatal("delta missed")
	}
	mismatch := questDefinitionFixture434()
	binary.LittleEndian.PutUint32(mismatch, 901)
	bad := questProgressController434{result: &QuestProgressObservation434{World: navFixture434(t).World}, selected: 900}
	if err := bad.observe(loginPacket434{op: 0x6936, body: mismatch}); err == nil {
		t.Fatal("definition mismatch")
	}
	bad.queryAt = 1
	if err := bad.tick(9000, send); err == nil {
		t.Fatal("timeout")
	}
	fail := questProgressController434{result: &o, readyAt: 1}
	if err := fail.tick(1500, func(uint16, []byte) error { return errors.New("send failed") }); err == nil {
		t.Fatal("send failure")
	}
}

func TestQuestProgress434CancelledAndUnknownInventory(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ObserveQuestProgress434(ctx, "TEST", make([]byte, 40), RealmInfo{Address: "127.0.0.1:1"}, "Synthetic", "127.0.0.1:1", 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	s := navFixture434(t).World.Store
	p := s.objects[17]
	p.Fields[0x1c0] = 88
	s.objects[88] = &Object434{GUID: 88, Created: true, Type: 2, Fields: map[uint16]uint32{8: 17, 0x4a: 1, 0x4c: 99}}
	s.objects[99] = &Object434{GUID: 99, Created: true, Type: 1, Fields: map[uint16]uint32{8: 17, FieldEntry434: 789, 0x10: 3}}
	if count, known := questInventoryCount434(&s, 789); !known || count != 3 {
		t.Fatal(count, known)
	}
	s.objects[88].Fields[0x4c] = 88
	if _, known := questInventoryCount434(&s, 789); known {
		t.Fatal("cyclic container accepted")
	}
	s.objects[88].Fields[0x4a] = 37
	if _, known := questInventoryCount434(&s, 789); known {
		t.Fatal("malformed slot count accepted")
	}
}
