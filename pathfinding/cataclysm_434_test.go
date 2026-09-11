package pathfinding

import (
	"encoding/binary"
	"os"
	"testing"
)

func TestCataclysm434TileBounds(t *testing.T) {
	for _, b := range [][]byte{nil, make([]byte, 100)} {
		if validateTile434(b) == nil {
			t.Fatal("bad tile accepted")
		}
	}
	b := make([]byte, 100)
	binary.LittleEndian.PutUint32(b, 0x444e4156)
	binary.LittleEndian.PutUint32(b[4:], 7)
	binary.LittleEndian.PutUint32(b[24:], 0xffffffff)
	if validateTile434(b) == nil {
		t.Fatal("count overflow accepted")
	}
	if NewMMapManager("synthetic").cataclysm434 {
		t.Fatal("default loader changed")
	}
}
func TestCataclysm434RuntimeRoute(t *testing.T) {
	dir := os.Getenv("CATAGHOST_MMAP_TEST_DIR")
	if dir == "" {
		t.Skip("optional read-only Cataclysm MMap test")
	}
	n := NewCataclysm434Navigator(dir)
	start := Point3D{10312.133, 831.909, 1326.410}
	dest := Point3D{10318.0, 827.9, 1326.410}
	r, err := n.FindPath(1, start, dest)
	if err != nil {
		t.Fatal(err)
	}
	if r.Type != PathfindNormal || len(r.Points) < 2 {
		t.Fatalf("route type=%x points=%v", r.Type, r.Points)
	}
	t.Logf("MMap route type=%x points=%d length=%.3f raw=%+v", r.Type, len(r.Points), r.PathLength(), r.Points)
}
