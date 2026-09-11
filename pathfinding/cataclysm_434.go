package pathfinding

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"github.com/o0olele/detour-go/detour"
)

// NewCataclysm434MMapManager opts into TCPP v14/20-byte tile headers. Default
// NewMMapManager remains AzerothCore v19. Both contain Detour v7/64-bit meshes.
func NewCataclysm434MMapManager(dir string) *MMapManager {
	m := NewMMapManager(dir)
	m.cataclysm434 = true
	return m
}

// Cataclysm434Navigator reuses PathFinder's polygon search, 4-unit smoothing and
// mesh height queries. It is session-owned and deliberately not concurrent.
type Cataclysm434Navigator struct {
	manager *MMapManager
	finders map[uint32]*PathFinder
}

func NewCataclysm434Navigator(mmapsDir string) *Cataclysm434Navigator {
	return &Cataclysm434Navigator{NewCataclysm434MMapManager(mmapsDir), make(map[uint32]*PathFinder)}
}
func (n *Cataclysm434Navigator) FindPath(mapID uint32, start, dest Point3D) (*PathResult, error) {
	pf := n.finders[mapID]
	if pf == nil {
		var err error
		pf, err = NewPathFinder(n.manager, nil, nil, mapID)
		if err != nil {
			return nil, err
		}
		pf.filter.SetIncludeFlags(1)
		pf.filter.SetExcludeFlags(0xfffe)
		n.finders[mapID] = pf
	}
	return pf.FindPath(start, dest)
}

// Detour AddTile uses unsafe views. Check all section lengths first using the
// build-compatible Go struct sizes (TCPP DetourNavMeshBuilder.cpp layout).
func validateTile434(b []byte) error {
	h := int(unsafe.Sizeof(detour.DtMeshHeader{}))
	if len(b) < h {
		return fmt.Errorf("truncated Detour header")
	}
	if binary.LittleEndian.Uint32(b) != uint32(detour.DT_NAVMESH_MAGIC) || binary.LittleEndian.Uint32(b[4:]) != 7 {
		return fmt.Errorf("invalid Detour magic/version")
	}
	total := uint64(h)
	for _, s := range []struct{ offset, size int }{{28, 12}, {24, int(unsafe.Sizeof(detour.DtPoly{}))}, {32, int(unsafe.Sizeof(detour.DtLink{}))}, {36, int(unsafe.Sizeof(detour.DtPolyDetail{}))}, {40, 12}, {44, 4}, {48, int(unsafe.Sizeof(detour.DtBVNode{}))}, {52, int(unsafe.Sizeof(detour.DtOffMeshConnection{}))}} {
		count := binary.LittleEndian.Uint32(b[s.offset:])
		total += (uint64(count)*uint64(s.size) + 3) &^ 3
		if total > uint64(len(b)) {
			return fmt.Errorf("Detour section count exceeds payload")
		}
	}
	if total != uint64(len(b)) {
		return fmt.Errorf("Detour payload size mismatch")
	}
	return nil
}
