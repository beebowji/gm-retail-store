package actions

import (
	"fmt"
	"strings"
	"time"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/timex"
	cuarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/cu-article-master"
)

func XGetDetailReprint(c *gwx.Context) (any, error) {
	reqNo := c.Param(`req_no`)
	arrReq := strings.Split(reqNo, `,`)

	reportList := []sqlx.Map{}
	var arrO, arrI []string
	for _, v := range arrReq {
		chkType := v[:1]
		if chkType == `I` {
			arrI = append(arrI, v)
		} else if chkType == `O` {
			arrO = append(arrO, v)
		}
	}

	sqlTemplateI := `SELECT msrr.req_no_recv as req_no,
	msrr.create_dtm as create_dtm,
	msr.article_id,
	msr.batch, 
	msr.serial,
	msr.mfg_date,
	msr.exp_date,
	msr.recv_qty as stock_qty,
	msr.recv_unit as base_unit,
	msr.bin_location as binloc
	FROM m_stock_req_recv msrr
	inner join m_stock_receive msr on msr.req_no_recv = msrr.req_no_recv 
	WHERE msrr.req_no_recv IN ('%v')`

	sqlTemplateO := `SELECT msrw.req_no_wd as req_no, 
	msrw.create_dtm as create_dtm,
	msw.article_id,
	msw.batch, 
	msw.serial,
	msw.mfg_date,
	msw.exp_date, 
	msw.wd_qty as stock_qty,
	msw.wd_unit as base_unit,
	msw.bin_location as binloc
	FROM m_stock_req_wd msrw
	inner join m_stock_withdraw msw on msrw.req_no_wd = msw.req_no_wd 
	WHERE msrw.req_no_wd IN ('%v')`

	reportI, err := processReport(arrI, sqlTemplateI, "ใบนำเข้า", "วันที่นำเข้าสินค้า", "เลขที่ใบนำเข้าสินค้า")
	if err != nil {
		return nil, err
	}
	reportList = append(reportList, reportI...)

	reportO, err := processReport(arrO, sqlTemplateO, "ใบเบิก", "วันที่เบิกสินค้า", "เลขที่ใบเบิกสินค้า")
	if err != nil {
		return nil, err
	}
	reportList = append(reportList, reportO...)

	resp := sqlx.Map{}
	resp.Set(`report_list`, reportList)
	return resp, nil
}

func processReport(reqList []string, queryTemplate string, typeReport string, datePrefix string, reqPrefix string) ([]sqlx.Map, error) {
	reportData := []sqlx.Map{}
	if len(reqList) == 0 {
		return reportData, nil
	}

	query := fmt.Sprintf(queryTemplate, strings.Join(reqList, `','`))
	rows, err := dbs.DH_RETAIL_STORE_R.QueryScan(query)
	if err != nil {
		return nil, err
	}

	saleRepresent, _ := cuarticlemaster.SALES_REPRESENTATIVE.GetTableR()

	if len(rows.Rows) == 0 {
		return reportData, nil
	}

	for _, req := range reqList {
		data := sqlx.Map{}
		data.Set(`type_report`, typeReport)
		data.Set(`date_now`, fmt.Sprintf(`วันที่พิมพ์ %v`, time.Now().Format(timex.DDsMMsYYYY)))

		createDtm := fmt.Sprintf(`%v %v`, datePrefix, rows.Rows[0].Time(`create_dtm`).Format(timex.DDsMMsYYYY))
		data.Set(`create_dtm`, createDtm)

		site := req[1:5]
		reqName := fmt.Sprintf(`%v %v/%v`, reqPrefix, site, req)
		data.Set(`req_no`, reqName)

		var articleList []sqlx.Map
		count := 1
		for _, row := range rows.Rows {
			if row.String(`req_no`) == req {
				mfgDate := ""
				if row.TimePtr(`mfg_date`) != nil {
					mfgDate = row.TimePtr(`mfg_date`).Local().Format(timex.YYYYMMDD)
				}

				expDate := ""
				if row.TimePtr(`exp_date`) != nil {
					expDate = row.TimePtr(`exp_date`).Local().Format(timex.YYYYMMDD)
				}

				rtName, mcName, articleName, unit := getProductDetails(row, saleRepresent)

				row.Set(`no`, count)
				row.Set("artcle_name", articleName)
				row.Set("rt_name", rtName)
				row.Set("mc_name", mcName)
				row.Set("base_unit", unit)
				row.Set("exp_date", expDate)
				row.Set("mfg_date", mfgDate)

				articleList = append(articleList, row)
				count++
			}
		}

		data.Set(`article_list`, articleList)
		reportData = append(reportData, data)
	}

	return reportData, nil
}

func getProductDetails(row sqlx.Map, saleRepresent *sqlx.Rows) (string, string, string, string) {
	rtName, mcName, artcleName, unit := "", "", "", ""
	mc_code := ""

	prod := cuarticlemaster.Products(row.String(`article_id`))
	if prod != nil {
		artcleName = prod.String(`name_th`)
		mc_code = prod.String(`merchandise_category2`)
	}

	mc := cuarticlemaster.ProductCategoriesMC(mc_code)
	if mc != nil {
		mcCode := mc.String("mc_code")
		if len(mcCode) > 3 {
			mcCode = mcCode[:3]
		}
		mcName = mcCode + " " + mc.String("mc_name_th")
	}

	if saleRepresent != nil {
		saleRepx := saleRepresent.FindRow(func(m *sqlx.Map) bool {
			return m.String("id") == prod.String(`zmm_seller`)
		})
		if saleRepx != nil {
			rtName = saleRepx.String(`seller_code`) + " " + saleRepx.String(`seller_name`)
		}
	}

	unitX := cuarticlemaster.UnitByKey(row.String(`base_unit`))
	if unitX != nil {
		unit = unitX.String(`name_th`)
	}

	return rtName, mcName, artcleName, unit
}
