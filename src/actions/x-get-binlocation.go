package actions

import (
	"fmt"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tablex"
)

type _X_GET_BINLOCATION_RQB struct {
	Site string `json:"site"`
	Sloc string `json:"sloc"`
}

func XGetBinlocation(c *gwx.Context) (any, error) {

	var dto _X_GET_BINLOCATION_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	// Validate
	if ex := c.Empty(dto.Site, `กรุณาระบุ Site`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(dto.Sloc, `กรุณาระบุ Sloc`); ex != nil {
		return nil, ex
	}

	// Connect
	dxRetailStore, ex := sqlx.ConnectPostgresRW(dbs.DH_COMPANY)
	if ex != nil {
		return nil, ex
	}

	qry := fmt.Sprintf(`select binloc from bin_location_master where werks = '%v' and	lgort = '%v'`, dto.Site, dto.Sloc)

	// Execute main query
	mainQuery := fmt.Sprintf(`(%s) as t`, qry)
	row, err := tablex.ExReport(c, dxRetailStore, mainQuery, ``)
	if err != nil {
		return nil, err
	}

	return row, nil

}
