package govinfo

import (
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/billstatus"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// parseStatus maps a BILLSTATUS document with the shared mapping.
func parseStatus(b []byte, src data.SourceInfo) (data.BillRecord, error) {
	rec, err := billstatus.Parse(b, src)
	if err != nil {
		return data.BillRecord{}, fmt.Errorf("govinfo: status: %w", err)
	}
	return rec, nil
}
