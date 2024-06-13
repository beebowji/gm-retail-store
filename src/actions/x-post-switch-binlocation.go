package actions

import (
	"fmt"
	"strings"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
	"gitlab.dohome.technology/dohome-2020/go-structx/sappix"
)

type X_GET_LIST_BINLOCATION_MASTER_RQB struct {
	Site       string                   `json:"site"`
	Sloc       string                   `json:"sloc"`
	BinlocList []BINLOC_LIST_SWITCH_RQB `json:"binloc_list"`
}

type BINLOC_LIST_SWITCH_RQB struct {
	BinLoc  string `json:"binloc"`
	Useflag string `json:"useflag"`
}

func XPostSwitchBinLocation(c *gwx.Context) (any, error) {

	var request X_GET_LIST_BINLOCATION_MASTER_RQB
	err := c.ShouldBindJSON(&request)
	if err != nil {
		return nil, err
	}

	// Validate request
	if validx.IsEmpty(request.Site) || validx.IsEmpty(request.Sloc) || len(request.BinlocList) == 0 {
		return nil, c.Error("Invalid request data").StatusBadRequest()
	}

	// Build binlocArr
	var binlocArr []string
	for _, v := range request.BinlocList {
		if validx.IsEmpty(v.BinLoc) {
			return nil, c.Error("Invalid BinLoc in request").StatusBadRequest()
		}
		if len(v.BinLoc) != 10 {
			return nil, c.Error("BinLoc must have 10 digits").StatusBadRequest()
		}
		binlocArr = append(binlocArr, v.BinLoc)
	}

	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_COMPANY)
	if ex != nil {
		return nil, ex
	}

	// Query bin locations
	query := fmt.Sprintf(`select binloc, count(bin_code) as total, aprvflag
	from bin_location_master blm 
	left join bin_location bl on blm.binloc = bl.bin_code and werks = bl.site and lgort = bl.sloc
	where werks = '%s' and lgort = '%s' AND binloc IN ('%s')
	GROUP BY binloc, aprvflag`, request.Site, request.Sloc, strings.Join(binlocArr, "','"))

	rows, err := dx.QueryScan(query)
	if err != nil {
		return nil, err
	}
	rows.BuildMap(func(m *sqlx.Map) string {
		return m.String(`binloc`)
	})

	saps := sappix.ZMMBAPI_CREATE_SLOC_RQB{}
	saps.IV_MODE = "U"
	var openLo, closeLo []string
	for _, v := range request.BinlocList {
		found := rows.FindMap(v.BinLoc)

		// เช็คว่าต้องการปิดหรือเปิด
		if v.Useflag == "X" { // เปิด
			if found.String(`aprvflag`) == "C" { // เช็คว่าตำแหน่งนี้อนุมัติรึยัง
				openLo = append(openLo, v.BinLoc)
			} else {
				return nil, c.Error(fmt.Sprintf("BinLoc %s is not approved", v.BinLoc)).StatusBadRequest()
			}
		} else if v.Useflag == "" { // ปิด
			if found.Int(`total`) == 0 { // เช็คว่ามีสินค้าจัดเก็บอยู่หรือไม่
				closeLo = append(closeLo, v.BinLoc)
			} else {
				return nil, c.Error(fmt.Sprintf("BinLoc %s contains items", v.BinLoc)).StatusBadRequest()
			}
		}

		// set sap
		item := sappix.ZMMBAPI_CREATE_SLOC_RQB_T_LOCSTRC{
			LOCZONE:   v.BinLoc[:1],
			LOCHSHELF: v.BinLoc[1:4],
			LOCSIDE:   v.BinLoc[4:5],
			LOCHOLE:   v.BinLoc[5:7],
			LOCCLASS:  v.BinLoc[7:9],
			LOCTYPE:   v.BinLoc[9:],
			WERKS:     request.Site,
			LGORT:     request.Sloc,
			BINLOC:    v.BinLoc,
			USEFLAG:   v.Useflag,
		}
		saps.T_LOCSTRC.Item = append(saps.T_LOCSTRC.Item, item)
	}

	// sap
	var resp *sappix.ZMMBAPI_CREATE_SLOC_RSB
	if len(saps.T_LOCSTRC.Item) > 0 {
		resp, ex = sappix.ZMMBAPI_CREATE_SLOC(nil, saps)
		if ex != nil {
			return nil, ex
		}

		for _, v := range resp.T_ERROR.Item {
			if v.TYPE != "S" {
				return nil, c.Error(fmt.Sprintf(`%v: %v`, v.LOCATION, v.MESSAGE)).StatusBadRequest()
			}
		}
	}

	if ex := dx.Transaction(func(t *sqlx.Tx) error {

		// update ปิดตำแหน่ง
		if len(closeLo) > 0 {
			sql := fmt.Sprintf(`update bin_location_master
			set useflag = ''
			where binloc IN ('%v')`, strings.Join(closeLo, `','`))
			_, ex = t.Exec(sql)
			if ex != nil {
				return ex
			}
		}

		// update เปิดตำแหน่ง
		if len(openLo) > 0 {
			sql := fmt.Sprintf(`update bin_location_master
			set useflag = 'X'
			where binloc IN ('%v')`, strings.Join(openLo, `','`))
			_, ex = t.Exec(sql)
			if ex != nil {
				return ex
			}
		}

		return nil
	}); ex != nil {
		return nil, ex
	}

	return nil, nil
}
