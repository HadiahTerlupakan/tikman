package services

import (
	"fmt"
	"sort"

	"github.com/google/uuid"
)

// FlowPeeringPOP is one POP's part of a destination, split over the
// upstreams that carried it to that POP. A nil POP is the traffic of
// addresses in no POP's blocks.
type FlowPeeringPOP struct {
	POPInterfaceID *uuid.UUID          `json:"pop_interface_id"`
	Bytes          int64               `json:"bytes"`
	Percent        float64             `json:"percent"`
	Upstreams      []FlowUpstreamShare `json:"upstreams"`
}

type popUpstreamCell struct {
	POPInterfaceID      *uuid.UUID `gorm:"column:pop_interface_id"`
	UpstreamInterfaceID uuid.UUID  `gorm:"column:upstream_interface_id"`
	Bytes               int64
}

func (s *FlowQueryService) popUpstreamBytes(table, where string, args []interface{}) ([]popUpstreamCell, error) {
	// table comes from flowSources, never from the request.
	query := fmt.Sprintf(`SELECT pop_interface_id, upstream_interface_id, %s AS bytes
		FROM %s WHERE %s GROUP BY 1, 2 ORDER BY 3 DESC, 2`, cappedBigint("SUM(bytes)"), table, where)
	var cells []popUpstreamCell
	return cells, s.db.Raw(query, args...).Scan(&cells).Error
}

// peeringPOPs groups the cells by POP, largest POP first; within a POP the
// upstreams keep the query's largest-first order.
func peeringPOPs(cells []popUpstreamCell, total int64) []FlowPeeringPOP {
	index := map[uuid.UUID]int{}
	pops := []FlowPeeringPOP{}
	for _, c := range cells {
		key := popOf(c.POPInterfaceID)
		i, ok := index[key]
		if !ok {
			i = len(pops)
			index[key] = i
			pops = append(pops, FlowPeeringPOP{POPInterfaceID: c.POPInterfaceID, Upstreams: []FlowUpstreamShare{}})
		}
		pops[i].Bytes = addBytes(pops[i].Bytes, c.Bytes)
		pops[i].Upstreams = append(pops[i].Upstreams,
			FlowUpstreamShare{UpstreamInterfaceID: c.UpstreamInterfaceID, Bytes: c.Bytes})
	}
	sort.SliceStable(pops, func(a, b int) bool { return pops[a].Bytes > pops[b].Bytes })
	for i := range pops {
		pops[i].Percent = percentOf(pops[i].Bytes, total)
	}
	return pops
}
