package actions

import (
	"fmt"
	"strings"
	"time"

	"gitlab.dohome.technology/dohome-2020/gm-retail-store/src/common"
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-structx/sappix"
)

type _X_POST_MOVE_ARTICLE_ALL_RQB struct {
	Site         string `json:"site"`
	Sloc         string `json:"sloc"`
	Perid        string `json:"perid"`
	BinlocOrigin string `json:"binloc_origin"`
	BinlocDes    string `json:"binloc_des"`
}

func XPostMoveArticleAll(c *gwx.Context) (any, error) {

	var dto _X_POST_MOVE_ARTICLE_ALL_RQB
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
	if ex := c.Empty(dto.Perid, `กรุณาระบุ Perid`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(dto.BinlocOrigin, `กรุณาระบุ BinlocOrigin`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(dto.BinlocDes, `กรุณาระบุ BinlocDes`); ex != nil {
		return nil, ex
	}

	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}
	dxCompany, ex := sqlx.ConnectPostgresRW(dbs.DH_COMPANY)
	if ex != nil {
		return nil, ex
	}

	err := getAvailableBinLocs(dxCompany, dto)

	if err != nil {
		return nil, err
	}

	// Get table
	mStockTable, ex := dx.TableEmpty(`m_stock`)
	if ex != nil {
		return nil, ex
	}
	mStockTransferTable, ex := dx.TableEmpty(`m_stock_transfer`)
	if ex != nil {
		return nil, ex
	}

	binDes, err := checkBinDes(dxCompany, dto)
	if err != nil {
		return nil, err
	}

	// query ข้อมูลสำหรับ map
	qryMStock := fmt.Sprintf(`select id, article_id, batch, serial, mfg_date, exp_date, stock_qty, base_unit
	from m_stock 
	where site = '%v' and sloc = '%v' and bin_location = '%v'`, dto.Site, dto.Sloc, dto.BinlocOrigin)
	rowMSock, ex := dx.QueryScan(qryMStock)
	if ex != nil {
		return nil, ex
	}
	if len(rowMSock.Rows) == 0 {
		return nil, c.Error(`ไม่พบตำแหน่งจัดเก็บในระบบ`).StatusBadRequest()
	}

	// Set SAP request for cancelling article binding
	sapsCancel := buildSAPRequest(dto, rowMSock, "D")
	respCancel, ex := sappix.ZDD_HH_PROCESS_ASSIGNLOC(nil, sapsCancel)
	if ex != nil {
		return nil, fmt.Errorf("failed to process SAP request for cancelling: %v", ex)
	}
	// Check SAP response for cancelling
	success, errMsg := common.IsSAPResponseSuccessful(respCancel)
	if !success {
		return nil, fmt.Errorf("SAP: เกิดข้อผิดพลาด (%v)", errMsg)
	}

	// Set SAP request for binding article to a new location
	sapsBind := buildSAPRequest(dto, rowMSock, "C")
	respBind, ex := sappix.ZDD_HH_PROCESS_ASSIGNLOC(nil, sapsBind)
	if ex != nil {
		return nil, fmt.Errorf("failed to process SAP request for binding: %v", ex)
	}
	// Check SAP response for binding
	success, errMsg = common.IsSAPResponseSuccessful(respBind)
	if !success {
		return nil, fmt.Errorf("SAP: เกิดข้อผิดพลาด (%v)", errMsg)
	}

	rowMSockDes, err := getExitStockDes(dx, dto)
	if err != nil {
		return nil, err
	}

	// update ฝั่งคลัง m ต่อ
	timeNow := time.Now()
	var binOriginDelete []string
	for _, v := range rowMSock.Rows {
		id := v.String(`id`)
		stockQty := v.Int64(`stock_qty`)

		if binDes == `` {
			// Moving to an empty location
			mStockTable.Rows = append(mStockTable.Rows, sqlx.Map{
				`id`:           id,
				`bin_location`: dto.BinlocDes,
				`stock_qty`:    stockQty,
				`update_dtm`:   timeNow,
				`remark`:       ``,
				`update_by`:    dto.Perid,
			})
		} else {
			// Moving to an existing location
			rowApp := mergStock(v, rowMSockDes, dto)
			if id != rowApp.String("id") {
				binOriginDelete = append(binOriginDelete, id)
			}
			rowApp.Set(`remark`, ``)
			mStockTable.Rows = append(mStockTable.Rows, rowApp)
		}

		// m_stock_transfer
		mStockTransferTable.Rows = append(mStockTransferTable.Rows, sqlx.Map{
			`create_by`:           dto.Perid,
			`article_id`:          v.String(`article_id`),
			`site`:                dto.Site,
			`sloc`:                dto.Sloc,
			`batch`:               v.String(`batch`),
			`serial`:              v.String(`serial`),
			`mfg_date`:            v.TimePtr(`mfg_date`),
			`exp_date`:            v.TimePtr(`exp_date`),
			`bin_location_origin`: dto.BinlocOrigin,
			`bin_location_des`:    dto.BinlocDes,
			`stock_qty`:           v.Float(`stock_qty`),
			`base_unit`:           v.String(`base_unit`),
			`create_dtm`:          timeNow,
		})
	}

	if ex := dxCompany.Transaction(func(t *sqlx.Tx) error {
		// Update bin_location or delete bin_origin based on binDes condition
		var query string
		if binDes != "" {
			query = fmt.Sprintf(`DELETE FROM bin_location WHERE site = '%v' AND sloc = '%v' AND bin_code = '%v'`, dto.Site, dto.Sloc, dto.BinlocOrigin)
		} else {
			query = fmt.Sprintf(`UPDATE bin_location SET bin_code = '%v' WHERE site = '%v' AND sloc = '%v' AND bin_code = '%v'`, dto.BinlocDes, dto.Site, dto.Sloc, dto.BinlocOrigin)
		}
		if _, ex := t.Exec(query); ex != nil {
			return ex
		}

		if ex = dx.Transaction(func(r *sqlx.Tx) error {
			// update m_stock
			if len(mStockTable.Rows) > 0 {
				colsConflict := []string{`id`}
				colsUpdate := []string{`bin_location`, `stock_qty`, `update_dtm`, `update_by`, `remark`}
				_, ex = r.InsertUpdateMany(`m_stock`, mStockTable, colsConflict, nil, colsUpdate, nil, 100)
				if ex != nil {
					return ex
				}
			}

			if len(binOriginDelete) > 0 {
				sqlDeleteOrigin := fmt.Sprintf(`delete from m_stock where id in ('%v')`, strings.Join(binOriginDelete, `','`))
				_, err := dx.Exec(sqlDeleteOrigin)
				if err != nil {
					return err
				}
			}

			// insert log m_stock_transfer
			if len(mStockTransferTable.Rows) > 0 {
				_, ex = r.InsertCreateBatches(`m_stock_transfer`, mStockTransferTable, 100)
				if ex != nil {
					return ex
				}
			}
			return nil
		}); ex != nil {
			return ex
		}

		return nil
	}); ex != nil {
		return nil, ex
	}

	return nil, nil
}

// Helper function to build SAP request
func buildSAPRequest(dto _X_POST_MOVE_ARTICLE_ALL_RQB, rowMSock *sqlx.Rows, mode string) sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB {
	saps := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB{
		IN_USERNAME:    dto.Perid,
		IN_TCODE:       "M_Stock",
		IN_STORAGE_LOC: dto.Sloc,
		IN_WERKS:       dto.Site,
		IV_MODE:        mode,
	}

	articleExists := make(map[string]bool)
	for _, v := range rowMSock.Rows {
		articleID := v.String(`article_id`)
		baseUnit := v.String(`base_unit`)
		key := articleID + baseUnit

		if articleExists[key] {
			continue
		}
		articleExists[key] = true

		item := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB_I_ARTICLE_ASSIGNLOC{
			IN_BIN_CODE:      getBinCode(mode, dto.BinlocOrigin, dto.BinlocDes),
			IN_MATNR:         articleID,
			IN_SITE:          dto.Site,
			IN_STORAGE_LOC:   dto.Sloc,
			IN_UNITOFMEASURE: baseUnit,
		}
		saps.I_ARTICLE_ASSIGNLOC.Item = append(saps.I_ARTICLE_ASSIGNLOC.Item, item)
	}

	return saps
}

// Helper function to get bin code based on mode
func getBinCode(mode, binOrigin, binDes string) string {
	if mode == "D" {
		return binOrigin
	}
	return binDes
}

func getAvailableBinLocs(dxCompany *sqlx.DB, dto _X_POST_MOVE_ARTICLE_ALL_RQB) error {
	//var binLocs []string

	if dto.BinlocOrigin[0:1] != "M" {
		return fmt.Errorf(`bin_location %v ไม่ได้อยู่ในคลัง M`, dto.BinlocOrigin)
	}

	if dto.BinlocDes[0:1] != "M" {
		return fmt.Errorf(`bin_location %v ไม่ได้อยู่ในคลัง M`, dto.BinlocDes)
	}

	query := `select binloc, useflag, aprvflag from bin_location_master bl 
	          where werks  = $1 and lgort  = $2  and binloc in ($3,$4)`
	rows, err := dxCompany.QueryScan(query, dto.Site, dto.Sloc, dto.BinlocDes, dto.BinlocOrigin)
	if err != nil {
		return err
	}

	if len(rows.Rows) == 0 {
		return fmt.Errorf(`ไม่พบ bin_location ทั้งต้นทางและปลายทาง`)
	}
	if len(rows.Rows) == 1 {

		if rows.Rows[0].String("binloc") == dto.BinlocOrigin {
			return fmt.Errorf(`ไม่พบ bin_location ต้นทาง`)
		}
		if rows.Rows[0].String("binloc") == dto.BinlocDes {
			return fmt.Errorf(`ไม่พบ bin_location ปลายทาง`)
		}
	}

	for _, v := range rows.Rows {
		if v.String("useflag") != "X" {
			return fmt.Errorf(`bin_location %v ไม่สามารถใช้ได้`, v.String("binloc"))
		}
		if v.String("aprvflag") != "C" {
			return fmt.Errorf(`bin_location %v ยังไม่ได้รับการอนุมัติ`, v.String("binloc"))
		}
	}
	return nil
}

func checkBinDes(dxCompany *sqlx.DB, dto _X_POST_MOVE_ARTICLE_ALL_RQB) (string, error) {
	query := `select bin_code from bin_location where site  = $1 and sloc  = $2  and bin_code = $3`
	rows, err := dxCompany.QueryScan(query, dto.Site, dto.Sloc, dto.BinlocDes)
	if err != nil {
		return "", err
	}

	if len(rows.Rows) == 0 {
		return "", nil
	}

	return rows.Rows[0].String("bin_code"), nil
}

func mergStock(stockOrigin sqlx.Map, rows *sqlx.Rows, dto _X_POST_MOVE_ARTICLE_ALL_RQB) sqlx.Map {

	for _, v := range rows.Rows {
		if v.String("batch") != `` && v.String("batch") == stockOrigin.String("batch") && v.String("article_id") == stockOrigin.String("article_id") {
			v.Set(`stock_qty`, v.Float(`stock_qty`)+stockOrigin.Float(`stock_qty`))
			return v
		}
		if v.String("serial") != `` && v.String("serial") == stockOrigin.String("serial") && v.String("article_id") == stockOrigin.String("article_id") {
			v.Set(`stock_qty`, v.Float(`stock_qty`)+stockOrigin.Float(`stock_qty`))
			return v
		}
		if v.String("exp_date") != `` && v.String("exp_date") == stockOrigin.String("exp_date") && v.String("article_id") == stockOrigin.String("article_id") {
			v.Set(`stock_qty`, v.Float(`stock_qty`)+stockOrigin.Float(`stock_qty`))
			return v
		}
		if v.String("serial") == `` && v.String("batch") == `` && v.String("exp_date") == `` && v.String("article_id") == stockOrigin.String("article_id") {
			v.Set(`stock_qty`, v.Float(`stock_qty`)+stockOrigin.Float(`stock_qty`))
			return v
		}
	}
	stockOrigin.Set(`bin_location`, dto.BinlocDes)
	return stockOrigin
}

func getExitStockDes(dx *sqlx.DB, dto _X_POST_MOVE_ARTICLE_ALL_RQB) (*sqlx.Rows, error) {
	qryMStock := fmt.Sprintf(`select id, article_id,bin_location, batch, serial, mfg_date, exp_date, stock_qty, base_unit
	from m_stock 
	where site = '%v' and sloc = '%v' and bin_location = '%v'`, dto.Site, dto.Sloc, dto.BinlocDes)
	rowMSock, ex := dx.QueryScan(qryMStock)
	if ex != nil {
		return nil, ex
	}
	return rowMSock, nil
}
