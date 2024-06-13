package actions

import (
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
	cuarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/cu-article-master"
	gmarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/gm-article-master"
	"gitlab.dohome.technology/dohome-2020/go-structx/sappix"
)

type _X_POST_RECV_STOCK_RQB struct {
	Site        string `json:"site"`
	Sloc        string `json:"sloc"`
	CreateBy    string `json:"create_by"`
	UpdateBy    string `json:"update_by"`
	ReceiveList []struct {
		ArticleID    string     `json:"article_id"`
		Batch        string     `json:"batch"`
		Serial       string     `json:"serial"`
		MfgDate      *time.Time `json:"mfg_date"`
		ExpDate      *time.Time `json:"exp_date"`
		BinLocation  string     `json:"bin_location"`
		StockQty     float64    `json:"stock_qty"`
		RecvQty      float64    `json:"recv_qty"`
		RecvUnit     string     `json:"recv_unit"`
		RecvReasonID string     `json:"recv_reason_id"`
		Remark       string     `json:"remark"`
	} `json:"receive_list"`
}

type _X_POST_RECV_STOCK_RSB struct {
	ReqNoRecv string `json:"req_no_recv"`
}

func XPostRecvStock(c *gwx.Context) (any, error) {

	var dto _X_POST_RECV_STOCK_RQB
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
	if ex := c.Empty(dto.CreateBy, `กรุณาระบุ CreateBy`); ex != nil {
		return nil, ex
	}

	var serialList []string

	for _, v := range dto.ReceiveList {
		if ex := c.Empty(v.ArticleID, `กรุณาระบุ ArticleID`); ex != nil {
			return nil, ex
		}
		if ex := c.Empty(v.BinLocation, `กรุณาระบุ BinLocation`); ex != nil {
			return nil, ex
		}
		if ex := c.Empty(v.RecvUnit, `กรุณาระบุ RecvUnit`); ex != nil {
			return nil, ex
		}
		if ex := c.Empty(v.RecvReasonID, `กรุณาระบุ RecvReasonID`); ex != nil {
			return nil, ex
		}
		if v.Serial != "" {
			serialList = append(serialList, v.Serial)
		}
	}

	if len(serialList) > 0 {
		serialListExist, ex := checkAvailableSerial(serialList)
		if ex != nil {
			return nil, ex
		}
		if serialListExist != `` {
			return nil, fmt.Errorf("serial %v ถูกจัดเก็บไปแล้ว", serialListExist)
		}
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
	mStockReceiveTable, ex := dx.TableEmpty(`m_stock_receive`)
	if ex != nil {
		return nil, ex
	}
	mStockReqRecvTable, ex := dx.TableEmpty(`m_stock_req_recv`)
	if ex != nil {
		return nil, ex
	}
	binLocationTable, ex := dxCompany.TableEmpty(`bin_location`)
	if ex != nil {
		return nil, ex
	}

	var articleInBinloc []string
	for _, v := range dto.ReceiveList {
		key := fmt.Sprintf(`('%v','%v')`, v.ArticleID, v.BinLocation)
		if !validx.IsContains(articleInBinloc, key) {
			articleInBinloc = append(articleInBinloc, key)
		}
	}

	// query m_stock
	qryMStock := fmt.Sprintf(`select id, site, sloc, article_id, bin_location, stock_qty, batch, serial, exp_date, create_dtm
	from m_stock where site = '%v' and sloc = '%v' and (article_id,bin_location) in (%v)`, dto.Site, dto.Sloc, strings.Join(articleInBinloc, `,`))
	rowMStock, ex := dx.QueryScan(qryMStock)
	if ex != nil {
		return nil, ex
	}

	// query bin_location
	qryBinLoc := fmt.Sprintf(`select id, article_id, bin_code, site, sloc from bin_location where (article_id,bin_code) in (%v) and site = '%v' and sloc = '%v'`, strings.Join(articleInBinloc, `,`), dto.Site, dto.Sloc)
	rowBinLoc, ex := dxCompany.QueryScan(qryBinLoc)
	if ex != nil {
		return nil, ex
	}

	// set sap
	saps := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB{}
	saps.IN_USERNAME = dto.CreateBy
	saps.IN_TCODE = "M_Stock"
	saps.IN_STORAGE_LOC = dto.Sloc
	saps.IN_WERKS = dto.Site
	saps.IV_MODE = "C"

	var rto []_X_POST_RECV_STOCK_RSB
	timeNow := time.Now()
	existingData := make(map[string]struct{})

	// gen req_no_wd
	ReqNoRecv := common.GenerateReqNo(dto.Site, "I")

	seqNo := 1
	for _, v := range dto.ReceiveList {
		var exp *time.Time
		articleId := v.ArticleID
		binLoc := v.BinLocation
		bindPosition := false

		// เช็คว่ามีในคลังแล้วหรือยัง
		var findArticle *sqlx.Map
		if !validx.IsEmpty(v.Batch) || !validx.IsEmpty(v.Serial) || v.ExpDate != nil || v.MfgDate != nil {
			column, fRow := "", ""
			switch {
			case !validx.IsEmpty(v.Batch):
				column, fRow = "batch", v.Batch
			case !validx.IsEmpty(v.Serial):
				column, fRow = "serial", v.Serial
			case v.ExpDate != nil:
				column, fRow = "exp_date", tox.String(v.ExpDate)
				exp = v.ExpDate
			case v.MfgDate != nil && v.ExpDate == nil:
				column, fRow = "mfg_date", tox.String(v.MfgDate)

				// เคสที่ส่งมาแต่วันที่ผลิตต้องคำนวณหาวันหมดอายุ stamp ลงไปด้วย
				exp = common.FindExpDate(articleId, v.MfgDate)
			}

			findArticle = rowMStock.FindRow(func(m *sqlx.Map) bool {
				return m.String(`article_id`) == articleId && m.String(`bin_location`) == binLoc && m.String(column) == fRow
			})
		} else {
			findArticle = rowMStock.FindRow(func(m *sqlx.Map) bool {
				return m.String(`article_id`) == articleId && m.String(`bin_location`) == binLoc
			})
		}

		id := uuid.New()
		resultQty := v.RecvQty
		createDate := timeNow
		updateDate := &timeNow

		// เตรียม base unit, find base unit by article
		findBaseUnit, ex := cuarticlemaster.ProductUnits(articleId)
		if ex != nil {
			return nil, ex
		}

		var baseUnit string
		for key, val := range findBaseUnit.Rowm {
			if val["6"] == true {
				parts := strings.Split(key, "|")
				if len(parts) > 1 {
					baseUnit = parts[1]
				}
			}
		}

		// เช็คว่าที่ FE ส่งมาเป็น base รึยัง ถ้ายังให้ convert
		if v.RecvUnit != baseUnit {
			// set data convert to base
			var rqbUnits gmarticlemaster.X_CONVERT_UNITS_RQB
			var rsbUnits gmarticlemaster.X_CONVERT_UNITS_RSB
			rqbUnits.Item = append(rqbUnits.Item, gmarticlemaster.X_CONVERT_UNITS_RQB_ITEM{
				ArticleId:  v.ArticleID,
				UnitCodeFr: v.RecvUnit,
				UnitCodeTo: baseUnit,
				UnitAmtFr:  1,
			})

			// convert unit
			if len(rqbUnits.Item) > 0 {
				ex := gms.GM_ARTICLE_MASTER.HttpPost(`product-units/convert-units`).PayloadJson(rqbUnits).Do().Struct(&rsbUnits)
				if ex != nil {
					return nil, ex
				}
			}

			if !rsbUnits.Item[0].Success {
				return nil, c.Error(`ไม่สามารถ convert unit ได้`).StatusBadRequest()
			}

			resultQty = tox.Float(rsbUnits.Item[0].UnitAmtTo) * v.RecvQty
		}

		// ถ้ามีแล้วให้อัพเดต
		if findArticle != nil {
			id = *findArticle.UUID(`id`)
			resultQty += findArticle.Float(`stock_qty`)
			createDate = findArticle.Time(`create_dtm`)
		} else { // ถ้าไม่มีให้ผูกตำแหน่ง
			bindPosition = true
			updateDate = nil
		}

		mStockTable.Rows = append(mStockTable.Rows, sqlx.Map{
			`id`:           id,
			`site`:         dto.Site,
			`sloc`:         dto.Sloc,
			`article_id`:   articleId,
			`batch`:        v.Batch,
			`serial`:       v.Serial,
			`mfg_date`:     v.MfgDate,
			`exp_date`:     exp,
			`bin_location`: v.BinLocation,
			`stock_qty`:    resultQty,
			`base_unit`:    baseUnit,
			`create_dtm`:   createDate,
			`update_dtm`:   updateDate,
			`update_by`:    dto.CreateBy,
			`remark`:       v.Remark,
		})

		// chk การผูกตำแหน่ง
		if bindPosition {
			paddedArticleID := strings.Repeat("0", 10) + articleId

			// sap
			item := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB_I_ARTICLE_ASSIGNLOC{
				IN_BIN_CODE:      v.BinLocation,
				IN_MATNR:         paddedArticleID,
				IN_SITE:          dto.Site,
				IN_STORAGE_LOC:   dto.Sloc,
				IN_UNITOFMEASURE: baseUnit,
			}
			saps.I_ARTICLE_ASSIGNLOC.Item = append(saps.I_ARTICLE_ASSIGNLOC.Item, item)

			findBinLoc := rowBinLoc.FindRow(func(m *sqlx.Map) bool {
				return m.String(`article_id`) == articleId && m.String(`bin_code`) == binLoc && m.String(`site`) == dto.Site && m.String(`sloc`) == dto.Sloc
			})

			// กรณีที่ยังไม่มี binLoc นั้น insert bin_location
			if findBinLoc == nil {
				// insert bin_location
				keyChk := articleId + "|" + baseUnit + "|" + v.BinLocation + "|" + dto.Site + "|" + dto.Sloc
				if _, exists := existingData[keyChk]; !exists {
					binLocationTable.Rows = append(binLocationTable.Rows, sqlx.Map{
						`id`:         uuid.New(),
						`article_id`: articleId,
						`base_unit`:  baseUnit,
						`bin_code`:   v.BinLocation,
						`sloc_type`:  v.BinLocation[len(v.BinLocation)-1:],
						`site`:       dto.Site,
						`sloc`:       dto.Sloc,
					})

					existingData[keyChk] = struct{}{}
				}
			}
		}

		// set m_stock_receive
		mStockReceiveTable.Rows = append(mStockReceiveTable.Rows, sqlx.Map{
			`id`:             uuid.New(),
			`req_no_recv`:    ReqNoRecv,
			`seq_no`:         seqNo,
			`create_by`:      dto.CreateBy,
			`article_id`:     articleId,
			`recv_qty`:       v.RecvQty,
			`recv_unit`:      v.RecvUnit,
			`recv_reason_id`: v.RecvReasonID,
			`remark`:         v.Remark,
			`site`:           dto.Site,
			`sloc`:           dto.Sloc,
			`batch`:          v.Batch,
			`serial`:         v.Serial,
			`mfg_date`:       v.MfgDate,
			`exp_date`:       v.ExpDate,
			`bin_location`:   v.BinLocation,
			`stock_qty`:      resultQty,
			`base_unit`:      baseUnit,
			`create_dtm`:     timeNow,
		})

		seqNo++
	}

	// set m_stock_req_recv
	mStockReqRecvTable.Rows = append(mStockReqRecvTable.Rows, sqlx.Map{
		`id`:          uuid.New(),
		`req_no_recv`: ReqNoRecv,
		`create_by`:   dto.CreateBy,
		`create_dtm`:  timeNow,
	})

	rto = append(rto, _X_POST_RECV_STOCK_RSB{
		ReqNoRecv: ReqNoRecv,
	})

	// to sap
	if len(saps.I_ARTICLE_ASSIGNLOC.Item) > 0 {
		resp, ex := sappix.ZDD_HH_PROCESS_ASSIGNLOC(nil, saps)
		if ex != nil {
			return nil, fmt.Errorf("failed to process SAP request for binding: %v", ex)
		}

		// Check SAP response for binding
		success, errMsg := common.IsSAPResponseSuccessful(resp)
		if !success {
			return nil, fmt.Errorf("SAP: เกิดข้อผิดพลาด (%v)", errMsg)
		}
	}

	// insert
	if ex := dxCompany.Transaction(func(t *sqlx.Tx) error {
		// bin_location
		if len(binLocationTable.Rows) > 0 {
			_, ex = t.InsertCreateBatches(`bin_location`, binLocationTable, 100)
			if ex != nil {
				return ex
			}
		}

		// retail_store
		if ex = dx.Transaction(func(r *sqlx.Tx) error {
			// m_stock
			if len(mStockTable.Rows) > 0 {
				colsConflict := []string{`id`}
				colsUpdate := []string{`stock_qty`, `update_dtm`, `update_by`, `remark`}
				_, ex = r.InsertUpdateMany(`m_stock`, mStockTable, colsConflict, nil, colsUpdate, nil, 100)
				if ex != nil {
					return ex
				}
			}
			// m_stock_req_recv
			if len(mStockReqRecvTable.Rows) > 0 {
				_, ex = r.InsertUpdateBatches(`m_stock_req_recv`, mStockReqRecvTable, []string{`id`}, 100)
				if ex != nil {
					return ex
				}
			}
			// m_stock_receive
			if len(mStockReceiveTable.Rows) > 0 {
				_, ex = r.InsertUpdateBatches(`m_stock_receive`, mStockReceiveTable, []string{`id`}, 100)
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

func checkAvailableSerial(serialList []string) (string, error) {

	serialListExist := ``
	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return "", ex
	}
	querySerial := `select * from m_stock where serial in ('` + strings.Join(serialList, `','`) + `')`
	rows, ex := dx.QueryScan(querySerial)
	if ex != nil {
		return "", ex
	}
	for _, v := range rows.Rows {
		if serialListExist == `` {
			serialListExist = v.String("serial")
		} else {
			serialListExist += `,` + v.String("serial")
		}
	}
	return serialListExist, nil
}
