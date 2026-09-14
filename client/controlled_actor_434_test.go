package client

import "testing"

func TestUncontrolledPlayer434(t *testing.T) {
	newStore := func() *ObjectStore434 {
		return &ObjectStore434{PlayerGUID: 3, objects: map[uint64]*Object434{3: {GUID: 3, Type: 4, Created: true, ThisIsYou: true, Fields: map[uint16]uint32{}}}}
	}
	if err := VerifyUncontrolledPlayer434(newStore()); err != nil {
		t.Fatal(err)
	}
	for _, field := range []uint16{8, 10, 14, 16} {
		s := newStore()
		s.objects[3].Fields[field+1] = 1
		if VerifyUncontrolledPlayer434(s) == nil {
			t.Fatalf("accepted high-word control field %d", field)
		}
	}
	for _, field := range []uint16{14, 16, 18} {
		s := newStore()
		s.objects[9] = &Object434{GUID: 9, Type: 3, Created: true, Fields: map[uint16]uint32{field: 3}}
		if VerifyUncontrolledPlayer434(s) == nil {
			t.Fatalf("accepted reverse control field %d", field)
		}
	}
	s := newStore()
	s.objects[3].Created = false
	if VerifyUncontrolledPlayer434(s) == nil {
		t.Fatal("accepted values-only player")
	}
	if VerifyUncontrolledPlayer434(nil) == nil {
		t.Fatal("accepted missing store")
	}
}
