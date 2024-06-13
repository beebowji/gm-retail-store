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

type _X_POST_MOVE_ARTICLE_ID_RQB struct {
	Site        string `json:"site"`
	Sloc        string `json:"sloc"`
	CreateBy    string `json:"create_by"`
	BinlocDes   string `json:"binloc_des"`
	ArticleList []struct {
		ArticleId    string     `json:"article_id"`
		ArticleName  string     `json:"article_name"`
		Batch        string     `json:"batch"`
		Serial       string     `json:"serial"`
		MfgDate      *time.Time `json:"mfg_date"`
		ExpDate      *time.Time `json:"exp_date"`
		Binloc       string     `json:"binloc"`
		QtyStock     float64    `json:"stock_qty"`
		BaseUnit     string     `json:"base_unit"`
		TrQty        float64    `json:"tr_qty"`
		Unit         string     `json:"unit"`
		RecvReasonId string     `json:"recv_reason_id"`
		Remark       string     `json:"remark"`
	} `json:"article_list"`
}

func XPostMoveArticleId(c *gwx.Context) (any, error) {

	var dto _X_POST_MOVE_ARTICLE_ID_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	// Validate the request
	if err := ValidateMoveArticleRequest(dto, c); err != nil {
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

	// Validate destination bin location
	if err := validateBinLocationUseflag(dto, dxCompany, c); err != nil {
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
	binLocationTable, ex := dxCompany.TableEmpty(`bin_location`)
	if ex != nil {
		return nil, ex
	}

	var article, baseUnit []string
	for _, v := range dto.ArticleList {
		if !validx.IsContains(article, v.ArticleId) {
			article = append(article, v.ArticleId)
		}
		if !validx.IsContains(baseUnit, v.BaseUnit) {
			baseUnit = append(baseUnit, v.BaseUnit)
		}
	}

	// chk ว่ามีตำแหน่งนี้ที่ bin_location มั้ย
	queryBin := fmt.Sprintf(`select * from bin_location 
	where site = '%v' and sloc = '%v' and bin_code = '%v' and article_id in ('%v') and base_unit in ('%v')`,
		dto.Site, dto.Sloc, dto.BinlocDes, strings.Join(article, `','`), strings.Join(baseUnit, `'%v'`))
	rowBinLog, ex := dxCompany.QueryScan(queryBin)
	if ex != nil {
		return nil, ex
	}
	rowBinLog.BuildMap(func(m *sqlx.Map) string {
		return fmt.Sprintf(`%s|%s|%s|%s|%s`, m.String(`article_id`), m.String(`base_unit`), m.String(`bin_code`), m.String(`site`), m.String(`sloc`))
	})

	// เก็บ article id ที่ต้องการย้าย
	var articleId []string
	var rqbUnits gmarticlemaster.X_CONVERT_UNITS_RQB
	var rsbUnits gmarticlemaster.X_CONVERT_UNITS_RSB
	if len(dto.ArticleList) > 0 {
		for _, v := range dto.ArticleList {
			if !validx.IsContains(articleId, v.ArticleId) {
				articleId = append(articleId, v.ArticleId)
			}

			// set data convert unit to base unit
			rqbUnits.Item = append(rqbUnits.Item, gmarticlemaster.X_CONVERT_UNITS_RQB_ITEM{
				ArticleId:  v.ArticleId,
				UnitCodeFr: v.Unit,
				UnitCodeTo: `-`,
				UnitAmtFr:  1,
			})
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
	queryMStock := fmt.Sprintf(`select id, site, sloc, article_id, batch, serial, mfg_date, exp_date, bin_location, stock_qty, base_unit, create_dtm, update_dtm, update_by
	from m_stock where article_id in ('%v') and site = '%v' and sloc = '%v'`, strings.Join(articleId, `','`), dto.Site, dto.Sloc)
	rowMStock, ex := dx.QueryScan(queryMStock)
	if ex != nil {
		return nil, ex
	}

	// set sap
	sapsForDel := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB{}
	sapsForDel.IN_USERNAME = dto.CreateBy
	sapsForDel.IN_TCODE = "M_Stock"
	sapsForDel.IN_STORAGE_LOC = dto.Sloc
	sapsForDel.IN_WERKS = dto.Site
	sapsForDel.IV_MODE = "D"

	sapsForCreat := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB{}
	sapsForCreat.IN_USERNAME = dto.CreateBy
	sapsForCreat.IN_TCODE = "M_Stock"
	sapsForCreat.IN_STORAGE_LOC = dto.Sloc
	sapsForCreat.IN_WERKS = dto.Site
	sapsForCreat.IV_MODE = "C"

	var delRowMStock, delRowBinCode, sapDel, sapCreat []string
	timeNow := time.Now()
	existingData := make(map[string]struct{})
	for _, v := range dto.ArticleList {
		id := v.ArticleId
		keySap := fmt.Sprintf(`%s|%s|%s`, v.Binloc, v.ArticleId, v.BaseUnit)

		// filter หา row ที่ต้องการย้าย
		data := rowMStock.Filter(func(m *sqlx.Map) bool {
			return m.String(`article_id`) == id && m.String(`bin_location`) == v.Binloc
		})

		// หา base ที่ convert แล้วเพื่อเอาจำนวนมาหักลบออกจากในคลัง
		var baseQty float64
		key := fmt.Sprintf(`%v|%v`, v.ArticleId, v.Unit)
		for _, b := range rsbUnits.Item {
			if !b.Success {
				return nil, errors.New(b.ErrorText)
			}

			textBase := fmt.Sprintf(`%v|%v`, b.ArticleId, b.UnitCodeFr)
			if textBase == key {
				baseQty = tox.Float(b.UnitAmtTo) * v.TrQty
				break
			}
		}

		var remaining float64
		if !validx.IsEmpty(v.Batch) || !validx.IsEmpty(v.Serial) || v.ExpDate != nil { // case batch serial exp
			// หาตัวที่ตรงกับลูปรอบนี้
			for _, row := range data.Rows {
				var match bool
				switch {
				case !validx.IsEmpty(v.Batch):
					match = row.String(`batch`) == v.Batch
				case !validx.IsEmpty(v.Serial):
					match = row.String(`serial`) == v.Serial
				case v.ExpDate != nil:
					match = row.TimePtr(`exp_date`).Equal(*v.ExpDate)
				}

				if match {
					remaining = row.Float(`stock_qty`) - baseQty

					// เก็บ id เพื่อลบ ยกเลิกผูกตำแหน่งจัดเก็บ
					if remaining == 0 {
						//key := fmt.Sprintf(`('%v','%v')`, row.String(`article_id`), row.String(`bin_location`))
						key := fmt.Sprintf(`('%v','%v','%v','%v')`,
							row.String(`article_id`),
							row.String(`bin_location`),
							row.String(`site`),
							row.String(`sloc`),
						)
						delRowBinCode = append(delRowBinCode, key)

						delRowMStock = append(delRowMStock, row.String(`id`))

						if !validx.IsContains(sapDel, keySap) {
							item := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB_I_ARTICLE_ASSIGNLOC{
								IN_BIN_CODE:      v.Binloc,
								IN_MATNR:         v.ArticleId,
								IN_SITE:          dto.Site,
								IN_STORAGE_LOC:   dto.Sloc,
								IN_UNITOFMEASURE: v.BaseUnit,
							}
							sapsForDel.I_ARTICLE_ASSIGNLOC.Item = append(sapsForDel.I_ARTICLE_ASSIGNLOC.Item, item)
							sapDel = append(sapDel, keySap)
						}

					} else if remaining > 0 { // ถ้าไม่เท่ากับ 0 ให้อัพเดตจำนวน row เดิม
						mStockTable.Rows = append(mStockTable.Rows, sqlx.Map{
							`id`:         row.UUID(`id`),
							`stock_qty`:  remaining,
							`update_dtm`: timeNow,
							`update_by`:  dto.CreateBy,
						})

					} else if remaining < 0 {
						return nil, fmt.Errorf("จำนวนที่ต้องการย้ายมากกว่าจำนวนที่มี")
					}

					break
				}
			}
		} else { // ถ้าไม่ใช่เคสที่เป็นประเภทอื่นจะมา row เดียว
			remaining = data.Rows[0].Float(`stock_qty`) - baseQty

			if remaining == 0 {
				key := fmt.Sprintf(`('%v','%v','%v','%v')`,
					data.Rows[0].String(`article_id`),
					data.Rows[0].String(`bin_location`),
					data.Rows[0].String(`site`),
					data.Rows[0].String(`sloc`),
				)
				delRowBinCode = append(delRowBinCode, key)

				delRowMStock = append(delRowMStock, data.Rows[0].String(`id`))

				if !validx.IsContains(sapDel, keySap) {
					item := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB_I_ARTICLE_ASSIGNLOC{
						IN_BIN_CODE:      v.Binloc,
						IN_MATNR:         v.ArticleId,
						IN_SITE:          dto.Site,
						IN_STORAGE_LOC:   dto.Sloc,
						IN_UNITOFMEASURE: v.BaseUnit,
					}
					sapsForDel.I_ARTICLE_ASSIGNLOC.Item = append(sapsForDel.I_ARTICLE_ASSIGNLOC.Item, item)
					sapDel = append(sapDel, keySap)
				}

			} else { // ถ้าไม่เท่ากับ 0 ให้อัพเดตจำนวน row เดิม
				mStockTable.Rows = append(mStockTable.Rows, sqlx.Map{
					`id`:           data.Rows[0].UUID(`id`),
					`article_id`:   data.Rows[0].String(`article_id`),
					`bin_location`: data.Rows[0].String(`bin_location`),
					`stock_qty`:    remaining,
					`update_dtm`:   timeNow,
					`update_by`:    dto.CreateBy,
				})
			}
		}

		// filter เช็ค article_id ที่ตำแหน่งใหม่
		column, fRow := "", ""
		var dataNewLog *sqlx.Map
		if !validx.IsEmpty(v.Batch) || !validx.IsEmpty(v.Serial) || v.ExpDate != nil {
			switch {
			case !validx.IsEmpty(v.Batch):
				column, fRow = "batch", v.Batch
			case !validx.IsEmpty(v.Serial):
				column, fRow = "serial", v.Serial
			case v.ExpDate != nil:
				column, fRow = "exp_date", tox.String(v.ExpDate)
			}

			dataNewLog = rowMStock.FindRow(func(m *sqlx.Map) bool {
				return m.String(`article_id`) == id && m.String(`bin_location`) == dto.BinlocDes && m.String(column) == fRow
			})
		} else {
			dataNewLog = rowMStock.FindRow(func(m *sqlx.Map) bool {
				return m.String(`article_id`) == id && m.String(`bin_location`) == dto.BinlocDes
			})
		}

		// เช็คก่อนว่าเคย set insert ที่ mStockTable รึยัง
		oldRow := mStockTable.FindRow(func(m *sqlx.Map) bool {
			return m.String(`article_id`) == id && m.String(`bin_location`) == dto.BinlocDes && m.String(column) == fRow
		})

		if oldRow != nil {
			oldRow.Set(`stock_qty`, oldRow.Float(`stock_qty`)+baseQty)
		} else {
			if dataNewLog != nil { // มีสินค้านี้ในตำแหน่งใหม่อยู่แล้ว ให้อัพเดตจำนวนที่ row เดิม
				mStockTable.Rows = append(mStockTable.Rows, sqlx.Map{
					`id`:           dataNewLog.UUID(`id`),
					`article_id`:   dataNewLog.String(`article_id`),
					`bin_location`: dataNewLog.String(`bin_location`),
					`stock_qty`:    dataNewLog.Float(`stock_qty`) + baseQty,
					`update_dtm`:   timeNow,
					`update_by`:    dto.CreateBy,
				})
			} else { // ไม่มีสินค้านี้ในตำแหน่งใหม่ ให้ผูกตำแหน่งก่อน
				// set sap
				if !validx.IsContains(sapCreat, keySap) {
					item := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB_I_ARTICLE_ASSIGNLOC{
						IN_BIN_CODE:      dto.BinlocDes,
						IN_MATNR:         v.ArticleId,
						IN_SITE:          dto.Site,
						IN_STORAGE_LOC:   dto.Sloc,
						IN_UNITOFMEASURE: v.BaseUnit,
					}
					sapsForCreat.I_ARTICLE_ASSIGNLOC.Item = append(sapsForCreat.I_ARTICLE_ASSIGNLOC.Item, item)
					sapCreat = append(sapCreat, keySap)
				}

				// chk ฝั่ง bin_location ก่อน ถ้ามีแล้วไม่ต้อง insert
				keyChk := v.ArticleId + "|" + v.BaseUnit + "|" + dto.BinlocDes + "|" + dto.Site + "|" + dto.Sloc
				found := rowBinLog.FindMap(keyChk)

				// set insert bin_location
				if _, exists := existingData[keyChk]; !exists && found == nil {
					binLocationTable.Rows = append(binLocationTable.Rows, sqlx.Map{
						`id`:         uuid.New(),
						`article_id`: v.ArticleId,
						`base_unit`:  v.BaseUnit,
						`bin_code`:   dto.BinlocDes,
						`sloc_type`:  dto.BinlocDes[len(dto.BinlocDes)-1:],
						`site`:       dto.Site,
						`sloc`:       dto.Sloc,
					})

					existingData[keyChk] = struct{}{}
				}

				mStockTable.Rows = append(mStockTable.Rows, sqlx.Map{
					`id`:           uuid.New(),
					`site`:         dto.Site,
					`sloc`:         dto.Sloc,
					`article_id`:   id,
					`batch`:        v.Batch,
					`serial`:       v.Serial,
					`mfg_date`:     v.MfgDate,
					`exp_date`:     v.ExpDate,
					`bin_location`: dto.BinlocDes,
					`stock_qty`:    baseQty,
					`base_unit`:    v.BaseUnit,
					`create_dtm`:   timeNow,
					`update_by`:    dto.CreateBy,
				})
			}
		}

		// เก็บ log m_stock_transfer
		mStockTransferTable.Rows = append(mStockTransferTable.Rows, sqlx.Map{
			`create_by`:           dto.CreateBy,
			`article_id`:          v.ArticleId,
			`tr_qty`:              v.TrQty,
			`tr_unit`:             v.Unit,
			`tr_reason_id`:        v.RecvReasonId,
			`remark`:              v.Remark,
			`site`:                dto.Site,
			`sloc`:                dto.Sloc,
			`batch`:               v.Batch,
			`serial`:              v.Serial,
			`mfg_date`:            v.MfgDate,
			`exp_date`:            v.ExpDate,
			`bin_location_origin`: v.Binloc,
			`bin_location_des`:    dto.BinlocDes,
			`stock_qty`:           baseQty,
			`base_unit`:           v.BaseUnit,
			`create_dtm`:          timeNow,
		})
	}

	// Saps
	if len(sapsForDel.I_ARTICLE_ASSIGNLOC.Item) > 0 {
		resp, ex := sappix.ZDD_HH_PROCESS_ASSIGNLOC(nil, sapsForDel)
		if ex != nil {
			return nil, ex
		}
		// Check SAP response for binding
		success, errMsg := common.IsSAPResponseSuccessful(resp)
		if !success {
			return nil, fmt.Errorf("SAP: เกิดข้อผิดพลาด (%v)", errMsg)
		}
	}
	if len(sapsForCreat.I_ARTICLE_ASSIGNLOC.Item) > 0 {
		resp, ex := sappix.ZDD_HH_PROCESS_ASSIGNLOC(nil, sapsForCreat)
		if ex != nil {
			return nil, ex
		}
		// Check SAP response for binding
		success, errMsg := common.IsSAPResponseSuccessful(resp)
		if !success {
			return nil, fmt.Errorf("SAP: เกิดข้อผิดพลาด (%v)", errMsg)
		}
	}

	if ex := dx.Transaction(func(t *sqlx.Tx) error {
		// delete m_stock
		if len(delRowMStock) > 0 {
			deleteQuery := fmt.Sprintf(`delete from m_stock where id in ('%v')`, strings.Join(delRowMStock, `','`))
			_, ex = t.Exec(deleteQuery)
			if ex != nil {
				return ex
			}
		}

		// update m_stock
		if len(mStockTable.Rows) > 0 {
			colsConflict := []string{`id`}
			colsUpdate := []string{`stock_qty`, `update_dtm`, `update_by`}
			_, ex = t.InsertUpdateMany(`m_stock`, mStockTable, colsConflict, nil, colsUpdate, nil, 100)
			if ex != nil {
				return ex
			}
		}

		// insert log m_stock_transfer
		if len(mStockTransferTable.Rows) > 0 {
			_, ex = t.InsertCreateBatches(`m_stock_transfer`, mStockTransferTable, 100)
			if ex != nil {
				return ex
			}
		}

		// dh_company
		if ex = dxCompany.Transaction(func(r *sqlx.Tx) error {
			if len(delRowBinCode) > 0 {
				// delete bin_location
				deleteQuery := fmt.Sprintf(`delete from bin_location where (article_id,bin_code,site,sloc) in (%v)`, strings.Join(delRowBinCode, `,`))
				_, ex = r.Exec(deleteQuery)
				if ex != nil {
					return ex
				}
			}

			// insert log bin_location
			if len(binLocationTable.Rows) > 0 {
				collConfict := []string{`article_id`, `bin_code`, `site`, `sloc`, `base_unit`}
				_, ex = r.InsertUpdateBatches(`bin_location`, binLocationTable, collConfict, 100)
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

func ValidateMoveArticleRequest(dto _X_POST_MOVE_ARTICLE_ID_RQB, c *gwx.Context) error {
	requiredFields := map[string]string{
		"Site":      dto.Site,
		"Sloc":      dto.Sloc,
		"CreateBy":  dto.CreateBy,
		"BinlocDes": dto.BinlocDes,
	}

	for field, value := range requiredFields {
		if ex := c.Empty(value, "กรุณาระบุ "+field); ex != nil {
			return ex
		}
	}

	for _, v := range dto.ArticleList {
		requiredArticleFields := map[string]any{
			"ArticleId":   v.ArticleId,
			"ArticleName": v.ArticleName,
			"Binloc":      v.Binloc,
			"BaseUnit":    v.BaseUnit,
			"TrQty":       v.TrQty,
		}
		for field, value := range requiredArticleFields {
			if ex := c.Empty(value, "กรุณาระบุ "+field); ex != nil {
				return ex
			}
		}

		if v.Binloc == dto.BinlocDes {
			return c.Error("unable to move to the original location.").StatusBadRequest()
		}
	}

	return nil
}

func validateBinLocationUseflag(dto _X_POST_MOVE_ARTICLE_ID_RQB, dxCompany *sqlx.DB, c *gwx.Context) error {
	queryLogMaster := fmt.Sprintf(
		`SELECT werks, lgort, binloc, useflag
		 FROM bin_location_master 
		 WHERE werks = '%v' AND lgort = '%v' AND binloc = '%v' AND useflag = 'X'`,
		dto.Site, dto.Sloc, dto.BinlocDes)

	rowLogMaster, err := dxCompany.QueryScan(queryLogMaster)
	if err != nil {
		return err
	}
	if len(rowLogMaster.Rows) == 0 {
		return c.Error(fmt.Sprintf("binloc %s has not been activated yet", dto.BinlocDes)).StatusBadRequest()
	}
	return nil
}
