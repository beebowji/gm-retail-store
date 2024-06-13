package actions

import (
	"fmt"
	"strings"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tablex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
)

func XGetListBinLocationMaster(c *gwx.Context) (any, error) {
	site, sloc, useflag, binlocList := c.Query("site"), c.Query("sloc"), c.Query("useflag"), c.Query("binloc_list")

	// Validate inputs
	if validx.IsEmpty(site) {
		return nil, c.Error(`กรุณาระบุ Site`).StatusBadRequest()
	}
	if validx.IsEmpty(sloc) {
		return nil, c.Error(`กรุณาระบุ Sloc`).StatusBadRequest()
	}

	// Prepare bin location list
	binlocArr := strings.FieldsFunc(binlocList, func(r rune) bool { return r == ',' || r == ' ' })
	binQuery := make([]string, 0, len(binlocArr))
	for _, v := range binlocArr {
		if v = strings.TrimSpace(v); v != "" {
			binQuery = append(binQuery, v)
		}
	}

	// Connect to the database
	dxRetailStore, ex := sqlx.ConnectPostgresRW(dbs.DH_COMPANY)
	if ex != nil {
		return nil, ex
	}

	// Build query
	query := fmt.Sprintf(`SELECT binloc, locwidth, lochigh, locdeep, useflag
	FROM bin_location_master
	WHERE substr(binloc, 1, 1) = 'M' AND aprvflag = 'C' AND werks = '%s' AND lgort = '%s'`, site, sloc)
	if len(binQuery) > 0 {
		query += fmt.Sprintf(" AND binloc IN ('%s')", strings.Join(binQuery, "','"))
	}
	switch useflag {
	case "X":
		query += " AND useflag = 'X'"
	case "N":
		query += " AND (useflag IS NULL OR useflag = '')"
	}

	rows, err := tablex.ExReport(c, dxRetailStore, fmt.Sprintf("(%s) as t", query), ``)
	if err != nil {
		return nil, err
	}
	if len(rows.Rows) == 0 {
		return nil, c.Error(`ไม่มีข้อมูลใน bin_location_master`).StatusBadRequest()
	}

	return rows, nil
}
