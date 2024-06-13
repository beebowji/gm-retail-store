package actions

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gitlab.dohome.technology/dohome-2020/gm-retail-store/src/common"
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gms"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tox"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
	gmarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/gm-article-master"
	"gitlab.dohome.technology/dohome-2020/go-structx/sappix"
)

type _X_POST_WD_STOCK_RQB struct {
	Site     string `json:"site"`
	Sloc     string `json:"sloc"`
	CreateBy string `json:"create_by"`
	UpdateBy string `json:"update_by"`
	WdList   []struct {
		ArticleID   string     `json:"article_id"`
		Batch       string     `json:"batch"`
		Serial      string     `json:"serial"`
		MfgDate     *time.Time `json:"mfg_date"`
		ExpDate     *time.Time `json:"exp_date"`
		BinLocation string     `json:"bin_location"`
		StockQty    float64    `json:"stock_qty"`
		BaseUnit    string     `json:"base_unit"`
		WdQty       int        `json:"wd_qty"`
		WdUnit      string     `json:"wd_unit"`
		WdReasonID  string     `json:"wd_reason_id"`
		Remark      string     `json:"remark"`
	} `json:"wd_list"`
}

type _X_POST_WD_STOCK_RSB struct {
	ReqNoWd string `json:"req_no_wd"`
}

func XPostWdStock(c *gwx.Context) (any, error) {

	var dto _X_POST_WD_STOCK_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	if err := validateDTOWDStock(&dto); err != nil {
		return nil, err
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

	// Get table
	mStockTable, ex := dx.TableEmpty(`m_stock`)
	if ex != nil {
		return nil, ex
	}
	mStockWdTable, ex := dx.TableEmpty(`m_stock_withdraw`)
	if ex != nil {
		return nil, ex
	}
	mStockReqWdTable, ex := dx.TableEmpty(`m_stock_req_wd`)
	if ex != nil {
		return nil, ex
	}

	// set data convert to base
	var rqbUnits gmarticlemaster.X_CONVERT_UNITS_RQB
	var rsbUnits gmarticlemaster.X_CONVERT_UNITS_RSB
	var articleId, binLoc []string
	for _, v := range dto.WdList {
		rqbUnits.Item = append(rqbUnits.Item, gmarticlemaster.X_CONVERT_UNITS_RQB_ITEM{
			ArticleId:  v.ArticleID,
			UnitCodeFr: v.WdUnit,
			UnitCodeTo: `-`,
			UnitAmtFr:  1,
		})

		if !validx.IsContains(articleId, v.ArticleID) {
			articleId = append(articleId, v.ArticleID)
		}
		if !validx.IsContains(binLoc, v.BinLocation) {
			binLoc = append(binLoc, v.BinLocation)
		}
	}
	// convert unit
	if len(rqbUnits.Item) > 0 {
		ex := gms.GM_ARTICLE_MASTER.HttpPost(`product-units/convert-units`).PayloadJson(rqbUnits).Do().Struct(&rsbUnits)
		if ex != nil {
			return nil, ex
		}
	}

	// query m_stock
	qryMStock := fmt.Sprintf(`select id, site, sloc , article_id, batch, serial, mfg_date, exp_date, bin_location, base_unit, stock_qty, create_dtm
	from m_stock 
	where article_id in ('%v') 
	and site = '%v' 
	and sloc = '%v' 
	and bin_location in ('%v')`, strings.Join(articleId, `','`), dto.Site, dto.Sloc, strings.Join(binLoc, `','`))
	rowMStock, ex := dx.QueryScan(qryMStock)
	if ex != nil {
		return nil, ex
	}

	rto, timeNow := make([]_X_POST_WD_STOCK_RSB, 0), time.Now()
	var delMStock []string

	saps := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB{}

	// gen req_no_wd
	reqNoWd := common.GenerateReqNo(dto.Site, "O")

	seqNo := 1
	for _, v := range dto.WdList {
		articleId := v.ArticleID
		binLoc := v.BinLocation
		var data *sqlx.Map

		if !validx.IsEmpty(v.Batch) || !validx.IsEmpty(v.Serial) || v.ExpDate != nil || v.MfgDate != nil {
			column, fRow := "", ""
			switch {
			case !validx.IsEmpty(v.Batch):
				column, fRow = "batch", v.Batch
			case !validx.IsEmpty(v.Serial):
				column, fRow = "serial", v.Serial
			case v.ExpDate != nil:
				column, fRow = "exp_date", tox.String(v.ExpDate)
			case v.MfgDate != nil && v.ExpDate == nil:
				column, fRow = "mfg_date", tox.String(v.MfgDate)
			}

			data = rowMStock.FindRow(func(m *sqlx.Map) bool {
				return m.String(`article_id`) == articleId && m.String(`bin_location`) == binLoc && m.String(column) == fRow
			})
		} else {
			data = rowMStock.FindRow(func(m *sqlx.Map) bool {
				return m.String(`article_id`) == v.ArticleID &&
					m.String(`bin_location`) == v.BinLocation
			})
		}

		//หา base ที่ convert แล้วเพื่อเอาจำนวนมาหักลบออกจากในคลัง
		var baseQty float64
		var baseUnit string
		key := fmt.Sprintf(`%v|%v`, v.ArticleID, v.WdUnit)
		for _, b := range rsbUnits.Item {
			textBase := fmt.Sprintf(`%v|%v`, b.ArticleId, b.UnitCodeFr)
			if textBase == key {
				baseQty = tox.Float(b.UnitAmtTo) * tox.Float(v.WdQty)
				baseUnit = b.UnitCodeTo
				break
			}
		}

		if data != nil {
			// หักลบจำนวนที่ต้องการจ่ายออกจากจำนวนใน DB
			resultQty := data.Float(`stock_qty`) - baseQty

			// ดักกรณีต้องการเบิกมากกว่าจำนวนที่มีในคลัง
			if resultQty < 0 {
				return nil, errors.New("จำนวนสินค้าที่เบิกจ่ายมากกว่าจำนวนสินค้าในคลัง")
			}

			// อัพเดตค่า stock_qty ใน rowMStock
			data.Set(`stock_qty`, resultQty)

			// เก็บ id ไว้เตรียมลบ
			if resultQty == 0 {
				delMStock = append(delMStock, data.String(`id`))
			} else { // stock != 0 ให้อัพเดต ที่ row เดิม
				mStockTable.Rows = append(mStockTable.Rows, sqlx.Map{
					`id`:           data.String(`id`),
					`site`:         data.String(`site`),
					`sloc`:         data.String(`sloc`),
					`article_id`:   data.String(`article_id`),
					`batch`:        data.String(`batch`),
					`serial`:       data.String(`serial`),
					`mfg_date`:     data.TimePtr(`mfg_date`),
					`exp_date`:     data.TimePtr(`exp_date`),
					`bin_location`: data.String(`bin_location`),
					`stock_qty`:    resultQty,
					`base_unit`:    data.String(`base_unit`),
					`create_dtm`:   data.TimePtr(`create_dtm`),
					`update_dtm`:   timeNow,
					`update_by`:    dto.UpdateBy,
				})
			}

			mStockWdTable.Rows = append(mStockWdTable.Rows, sqlx.Map{
				`id`:           uuid.New(),
				`req_no_wd`:    reqNoWd,
				`seq_no`:       seqNo,
				`create_by`:    dto.UpdateBy,
				`article_id`:   v.ArticleID,
				`wd_qty`:       v.WdQty,
				`wd_unit`:      v.WdUnit,
				`wd_reason_id`: v.WdReasonID,
				`remark`:       v.Remark,
				`site`:         dto.Site,
				`sloc`:         dto.Sloc,
				`batch`:        v.Batch,
				`serial`:       v.Serial,
				`mfg_date`:     v.MfgDate,
				`exp_date`:     v.ExpDate,
				`bin_location`: v.BinLocation,
				`stock_qty`:    baseQty,
				`base_unit`:    baseUnit,
				`create_dtm`:   timeNow,
			})

			seqNo++
		}
	}

	mStockReqWdTable.Rows = append(mStockReqWdTable.Rows, sqlx.Map{
		`id`:         uuid.New(),
		`req_no_wd`:  reqNoWd,
		`create_by`:  dto.UpdateBy,
		`create_dtm`: timeNow,
	})

	rto = append(rto, _X_POST_WD_STOCK_RSB{
		ReqNoWd: reqNoWd,
	})

	// ลูปเช็คสินค้าในตำแหน่งว่าเป็น 0 หมดหรือไม่ เพื่อยกเลิกผูกตำแหน่ง
	var delcode []string
	keyMap := make(map[string]bool)
	for _, v := range rowMStock.Rows {
		key := fmt.Sprintf("('%v','%v')", v.String(`article_id`), v.String(`bin_location`))

		if keyMap[key] {
			continue
		}
		keyMap[key] = true

		data := rowMStock.Filter(func(m *sqlx.Map) bool {
			return m.String(`article_id`) == v.String(`article_id`) && m.String(`bin_location`) == v.String(`bin_location`)
		})

		if data != nil {
			isAllZero := true
			for _, dataRow := range data.Rows {
				if dataRow.Float(`stock_qty`) != 0 {
					isAllZero = false
					break
				}
			}

			// If all quantities are zero, append the key to delcode
			if isAllZero {
				delcode = append(delcode, key)

				// set sap
				saps.IN_USERNAME = dto.UpdateBy
				saps.IN_TCODE = "M_Stock"
				saps.IN_STORAGE_LOC = dto.Sloc
				saps.IN_WERKS = dto.Site
				saps.IV_MODE = "D"
				item := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB_I_ARTICLE_ASSIGNLOC{
					IN_BIN_CODE:      data.Rows[0].String(`bin_location`),
					IN_MATNR:         data.Rows[0].String(`article_id`),
					IN_SITE:          dto.Site,
					IN_STORAGE_LOC:   dto.Sloc,
					IN_UNITOFMEASURE: data.Rows[0].String(`base_unit`),
				}
				saps.I_ARTICLE_ASSIGNLOC.Item = append(saps.I_ARTICLE_ASSIGNLOC.Item, item)
			}
		}
	}

	// Saps
	if len(saps.I_ARTICLE_ASSIGNLOC.Item) > 0 {
		resp, ex := sappix.ZDD_HH_PROCESS_ASSIGNLOC(nil, saps)
		if ex != nil {
			return nil, ex
		}
		success, errMsg := common.IsSAPResponseSuccessful(resp)
		if !success {
			return nil, fmt.Errorf("SAP: เกิดข้อผิดพลาด (%v)", errMsg)
		}

		// chk กรณีไม่มีการตอบกลับจาก SAP
		if len(resp.TBL_LOCATION.Item) == 0 && len(saps.I_ARTICLE_ASSIGNLOC.Item) > 0 {
			return nil, fmt.Errorf("SAP : เกิดข้อผิดพลาด ไม่สามารถลบตำแหน่งได้")
		}
	}

	if ex := dxCompany.Transaction(func(t *sqlx.Tx) error {
		// Delete bin_location
		if len(delcode) != 0 {
			delQuery := fmt.Sprintf(`delete from bin_location where (article_id,bin_code) in (%s) and site = '%v' and sloc = '%v'`, strings.Join(delcode, `,`), dto.Site, dto.Sloc)
			_, ex := t.Exec(delQuery)
			if ex != nil {
				return ex
			}
		}

		if ex := dx.Transaction(func(r *sqlx.Tx) error {
			// del m_stock
			if len(delMStock) > 0 {
				delQuery := fmt.Sprintf(`delete from m_stock where id in ('%s')`, strings.Join(delMStock, `','`))
				_, ex = r.Exec(delQuery)
				if ex != nil {
					return ex
				}
			}

			// insert
			if len(mStockReqWdTable.Rows) > 0 {
				_, ex = r.InsertCreateBatches(`m_stock_req_wd`, mStockReqWdTable, 100)
				if ex != nil {
					return ex
				}
			}
			if len(mStockWdTable.Rows) > 0 {
				_, ex = r.InsertCreateBatches(`m_stock_withdraw`, mStockWdTable, 100)
				if ex != nil {
					return ex
				}
			}

			// update
			if len(mStockTable.Rows) > 0 {
				_, ex = r.InsertUpdateBatches(`m_stock`, mStockTable, []string{`id`}, 100)
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

	return rto, nil

}

func validateDTOWDStock(dto *_X_POST_WD_STOCK_RQB) error {
	requiredFields := map[string]string{
		dto.Site:     "Site",
		dto.Sloc:     "Sloc",
		dto.CreateBy: "CreateBy",
		dto.UpdateBy: "UpdateBy",
	}

	for field, fieldName := range requiredFields {
		if field == "" {
			return fmt.Errorf("กรุณาระบุ %s", fieldName)
		}
	}

	for _, v := range dto.WdList {
		if v.ArticleID == "" {
			return errors.New("กรุณาระบุ ArticleID")
		}
		if v.BinLocation == "" {
			return errors.New("กรุณาระบุ BinLocation")
		}
		if v.StockQty == 0 {
			return errors.New("กรุณาระบุ StockQty")
		}
		if v.BaseUnit == "" {
			return errors.New("กรุณาระบุ BaseUnit")
		}
		if v.WdQty == 0 {
			return errors.New("กรุณาระบุ WdQty")
		}
		if v.WdReasonID == "" {
			return errors.New("กรุณาระบุ WdReasonID")
		}
	}
	return nil
}
