package actions

import (
	"fmt"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tablex"
)

func XGetListReasonV2(c *gwx.Context) (any, error) {

	reasonType := c.Query("reason_type")

	// Validate
	if ex := c.Empty(reasonType, `กรุณาระบุ ReasonType`); ex != nil {
		return nil, ex
	}

	// Connect
	dxRetailStore, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}

	// Query
	qry := fmt.Sprintf(`select reason_id, reason_type, reason_name, use_flag, vr_tran, vr_often, vr_wh, vr_snm from m_stock_reason where reason_type = '%v' order by reason_id asc`, reasonType)

	// Execute main query
	mainQuery := fmt.Sprintf(`(%s) as t`, qry)
	row, err := tablex.ExReport(c, dxRetailStore, mainQuery, ``)
	if err != nil {
		return nil, err
	}

	return row, nil
}
