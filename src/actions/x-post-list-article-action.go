package actions

import (
	"fmt"
	"strings"
	"time"

	"gitlab.dohome.technology/dohome-2020/gm-retail-store/src/common"
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tablex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/timex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
	cuarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/cu-article-master"
	cucompany "gitlab.dohome.technology/dohome-2020/go-structx/cu-company"
)

type _X_POST_LIST_ARTICLE_ACTION_RQB struct {
	SiteSloc []struct {
		Site string `json:"site"`
		Sloc string `json:"sloc"`
	} `json:"site_sloc"`
	ReportType   []string   `json:"report_type"`
	BeginDate    *time.Time `json:"begin_date"`
	EndDate      *time.Time `json:"end_date"`
	ArticleID    []string   `json:"article_id"`
	Binloc       string     `json:"binloc"`
	McCode       []string   `json:"mc_code"`
	RtCode       []string   `json:"rt_code"`
	Serial       string     `json:"serial"`
	Batch        string     `json:"batch"`
	ReasonID     []string   `json:"reason_id"`
	IsExportFile string     `json:"is_export_file"`
}

func XPostListArticleAction(c *gwx.Context) (any, error) {
	var dto _X_POST_LIST_ARTICLE_ACTION_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	// validate
	var siteSloc []string
	for _, v := range dto.SiteSloc {
		if ex := c.Empty(v.Site, `กรุณาระบุ Site`); ex != nil {
			return nil, ex
		}
		if ex := c.Empty(v.Sloc, `กรุณาระบุ Slocs`); ex != nil {
			return nil, ex
		}

		// key for qry
		key := fmt.Sprintf(`('%v','%v')`, v.Site, v.Sloc)
		siteSloc = append(siteSloc, key)
	}
	if len(dto.ReportType) == 0 {
		return nil, c.Error(`กรุณาระบุ ReportType`).StatusBadRequest()
	}
	if dto.BeginDate == nil {
		return nil, c.Error(`กรุณาระบุ BeginDate`).StatusBadRequest()
	}
	if dto.EndDate == nil {
		return nil, c.Error(`กรุณาระบุ EndDate`).StatusBadRequest()
	}

	// Connect
	dxRetailStore, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}

	qry := buildQueryInAction(dto, siteSloc)

	mainQuery := fmt.Sprintf(`(%s) as t`, qry)
	rows, err := tablex.ExReport(c, dxRetailStore, mainQuery, ``)
	if err != nil {
		return nil, err
	}

	rowReportItems := parseRowsinAction(rows)

	if !validx.IsEmpty(dto.IsExportFile) {
		columns := []string{"ลำดับ", "site", "sloc", "ประเภทรายงาน", "วันที่เคลื่อนย้าย", "เวลา", "ผู้ดำเนินการ", "ตำแหน่งต้นทาง", "ตำแหน่งปลายทาง", "ผู้ดูแลขาย", "หมวดหมู่สินค้า", "รหัสสินค้า", "ชื่อสินค้า", "จำนวน", "หน่วย", "Batch", "Serial", "วันที่ผลิต", "วันหมดอายุ", "อายุสินค้าคงเหลือ(วัน)", "เลขที่ Request", "เหตุผล", "หมายเหตุ"}
		excelURL, err := common.ExportExcelRpl("รายงานการเคลื่อนย้ายสินค้า", rowReportItems, columns)
		if err != nil {
			return nil, fmt.Errorf("failed to export data to Excel: %v", err)
		}
		return excelURL, nil
	}

	return rows, nil
}

func buildQueryInAction(dto _X_POST_LIST_ARTICLE_ACTION_RQB, siteSloc []string) string {
	// Convert each string element to lowercase
	for i, str := range dto.ReportType {
		dto.ReportType[i] = strings.ToLower(str)
	}

	// string query
	selectStr := `select sr.seller_code as rt_code,
	sr.seller_name as rt_name,
	pc.mc_code as mc_code,
	pc.mc_name_th as mc_name,
	p.name_th as article_name,`

	joinStr := `
	left join dh_article_master.product_categories pc on pc.mc_code = p.merchandise_category2
	left join dh_article_master.sales_representative sr on sr.id = p.zmm_seller`

	queryCaseO := `'เบิกจ่าย' as report_type,
	msw.create_dtm as create_date,
	msw.create_by as per_name,
	msw.site as site,
	msw.sloc as sloc,
	msw.bin_location as binloc_origin,
	'-' as binloc_des,
	msw.article_id as article_id,
	msw.wd_qty as stock_qty,
	msw.wd_unit as base_unit,
	msw.batch as batch,
	msw.serial as serial,
	msw.req_no_wd as req_no,
	msw.remark as remark,
	msw.exp_date as exp_date,
	msw.mfg_date as mfg_date,
	msr.reason_name as reason_name,
	msr.reason_id as reason_id
	FROM m_stock_withdraw msw
	inner join m_stock_reason msr on msw.wd_reason_id = msr.reason_id
	inner join dh_article_master.products p on msw.article_id = p.article_id`

	queryCaseI := `'นำเข้า' as report_type,
	msrecv.create_dtm as create_date,
	msrecv.create_by as per_name,
	msrecv.site as site,
	msrecv.sloc as sloc,
	'-' as binloc_origin,
	msrecv.bin_location as binloc_des,
	msrecv.article_id as article_id,
	msrecv.recv_qty as stock_qty,
	msrecv.recv_unit as base_unit,
	msrecv.batch as batch,
	msrecv.serial as serial,
	msrecv.req_no_recv as req_no,
	msrecv.remark as remark,
	msrecv.exp_date as exp_date,
	msrecv.mfg_date as mfg_date,
	msr.reason_name as reason_name,
	msr.reason_id as reason_id
	FROM m_stock_receive msrecv
	inner join m_stock_reason msr on msrecv.recv_reason_id = msr.reason_id
	inner join dh_article_master.products p on msrecv.article_id = p.article_id`

	queryCaseTR := `'ย้ายสินค้า' as report_type,
	mst.create_dtm as create_date,
	mst.create_by as per_name,
	mst.site as site,
	mst.sloc as sloc,
	mst.bin_location_origin as binloc_origin,
	mst.bin_location_des as binloc_des,
	mst.article_id as article_id,
	mst.tr_qty as stock_qty,
	mst.tr_unit as base_unit,
	mst.batch as batch,
	mst.serial as serial,
	'-' as req_no, 
	mst.remark as remark,
	mst.exp_date as exp_date,
	mst.mfg_date as mfg_date,
	NULL as reason_name, 
    NULL as reason_id 
	FROM m_stock_transfer mst
	inner join dh_article_master.products p on mst.article_id = p.article_id`

	// Initialize query parts for each case
	queryCaseO = buildQueryCase(dto, selectStr, joinStr, buildQueryWherePart(dto, siteSloc, "o"), queryCaseO)
	queryCaseI = buildQueryCase(dto, selectStr, joinStr, buildQueryWherePart(dto, siteSloc, "i"), queryCaseI)
	queryCaseTR = buildQueryCase(dto, selectStr, joinStr, buildQueryWherePart(dto, siteSloc, "tr"), queryCaseTR)

	// Combine query parts based on report type
	qry := combineQueryParts(dto, queryCaseO, queryCaseI, queryCaseTR)

	// Add finishing touches
	finishQry := addFinishingTouches(dto, qry)

	return finishQry
}

func buildQueryWherePart(dto _X_POST_LIST_ARTICLE_ACTION_RQB, siteSloc []string, caseStr string) string {
	// Initialize where conditions
	whereConditions := []string{
		fmt.Sprintf("(site,sloc) in (%v)", strings.Join(siteSloc, ",")),
		fmt.Sprintf("(create_dtm at time zone 'Asia/Bangkok')::date between '%v' and '%v'",
			dto.BeginDate.Local().Format(timex.YYYYMMDD), dto.EndDate.Local().Format(timex.YYYYMMDD)),
	}
	// Append additional conditions
	if len(dto.ArticleID) > 0 {
		whereConditions = append(whereConditions, fmt.Sprintf("p.article_id in ('%v')", strings.Join(dto.ArticleID, "','")))
	}
	if len(dto.McCode) > 0 {
		whereConditions = append(whereConditions, fmt.Sprintf("pc.mc_code in ('%v')", strings.Join(dto.McCode, "','")))
	}
	if len(dto.RtCode) > 0 {
		whereConditions = append(whereConditions, fmt.Sprintf("sr.seller_code in ('%v')", strings.Join(dto.RtCode, "','")))
	}
	if len(dto.ReasonID) > 0 && caseStr != "tr" {
		whereConditions = append(whereConditions, fmt.Sprintf("msr.reason_id in ('%v')", strings.Join(dto.ReasonID, "','")))
	}
	if !validx.IsEmpty(dto.Serial) {
		whereConditions = append(whereConditions, fmt.Sprintf("serial = '%v'", dto.Serial))
	}
	if !validx.IsEmpty(dto.Batch) {
		whereConditions = append(whereConditions, fmt.Sprintf("batch = '%v'", dto.Batch))
	}
	// Combine where conditions
	wherePart := " WHERE " + strings.Join(whereConditions, " AND ")
	return wherePart
}

func buildQueryCase(dto _X_POST_LIST_ARTICLE_ACTION_RQB, selectStr, joinStr, queryWherePart, queryCase string) string {
	// Initialize query case
	query := fmt.Sprintf("%v\n%v\n%v\n%v", selectStr, queryCase, joinStr, queryWherePart)
	// Append binloc conditions
	query = appendBinLocConditions(dto, queryCase, query)
	return query
}

func appendBinLocConditions(dto _X_POST_LIST_ARTICLE_ACTION_RQB, reportType, query string) string {
	if !validx.IsEmpty(dto.Binloc) {
		switch reportType {
		case "o":
			return query + fmt.Sprintf(" AND msw.bin_location = '%v'", dto.Binloc)
		case "i":
			return query + fmt.Sprintf(" AND msrecv.bin_location = '%v'", dto.Binloc)
		case "tr":
			return query + fmt.Sprintf(" AND mst.bin_location_origin = '%v'", dto.Binloc)
		}
	}
	return query
}

func combineQueryParts(dto _X_POST_LIST_ARTICLE_ACTION_RQB, queryCaseO, queryCaseI, queryCaseTR string) string {
	queryParts := make(map[string]string)
	queryParts["o"] = queryCaseO
	queryParts["i"] = queryCaseI
	queryParts["tr"] = queryCaseTR

	var parts []string
	if validx.IsContains(dto.ReportType, "all") {
		// Include all query parts
		for _, query := range queryParts {
			parts = append(parts, query)
		}
	} else {
		// Include only selected query parts
		for reportType, query := range queryParts {
			if validx.IsContains(dto.ReportType, reportType) {
				parts = append(parts, query)
			}
		}
	}
	return strings.Join(parts, "\nUNION ALL\n")
}

func addFinishingTouches(dto _X_POST_LIST_ARTICLE_ACTION_RQB, qry string) string {
	if isOnlySiteSlocInAction(dto) {
		return qry + " ORDER BY create_date DESC LIMIT 10"
	}
	return fmt.Sprintf("WITH stock_data AS (%v) SELECT * FROM stock_data order by create_date", qry)
}

func isOnlySiteSlocInAction(dto _X_POST_LIST_ARTICLE_ACTION_RQB) bool {
	return len(dto.ArticleID) == 0 &&
		dto.Binloc == "" &&
		len(dto.McCode) == 0 &&
		len(dto.RtCode) == 0 &&
		dto.Serial == "" &&
		dto.Batch == "" &&
		len(dto.ReasonID) == 0
}

func parseRowsinAction(rows *tablex.ReadR) *sqlx.Rows {
	rowReportItems := sqlx.NewRows()

	numRow := 1
	for _, v := range rows.Rows {
		// Format create_date to date and time strings
		if v.TimePtr("create_date") != nil {
			createDate := v.TimePtr("create_date").Local()
			dateString := createDate.Format("2006-01-02")
			timeString := createDate.Format("15:04:05")
			v.Set("create_date", dateString)
			v.Set("create_time", timeString)
		}

		// find ShelfLift
		v.Set(`shelf_lift`, "")
		if v.TimePtr(`exp_date`) != nil {
			expLocal := v.TimePtr(`exp_date`).Local()
			v.Set(`exp_date`, expLocal.Format("2006-01-02"))
			v.Set(`shelf_lift`, common.CalculateDaysDiff(v.TimePtr(`exp_date`)))
		}

		if v.TimePtr(`mfg_date`) != nil {
			mfgLocal := v.TimePtr(`mfg_date`).Local()
			v.Set(`mfg_date`, mfgLocal.Format("2006-01-02"))
		}

		// convert to unit_name
		baseUnitName := cuarticlemaster.UnitByKey(v.String(`base_unit`))
		v.Set(`base_unit`, baseUnitName.String(`name_th`))

		personName := ``
		person := cucompany.EmployeesX(v.String("per_name"))
		if person != nil {
			personName = ` ` + person.String("first_name") + ` ` + person.String("last_name")
		}
		v.Set(`per_name`, v.String(`per_name`)+personName)

		v.Set(`rt_name`, v.String("rt_code")+" "+v.String(`rt_name`))
		v.Set(`mc_name`, v.String("mc_code")+" "+v.String(`mc_name`))

		v.Set(`no`, numRow)
		numRow++
	}

	rowReportItems.Rows = append(rowReportItems.Rows, rows.Rows...)

	rowReportItems.Columns = append(rowReportItems.Columns, `No`, `Site`, `Sloc`, `ReportType`, `CreateDate`, `CreateTime`, `PerName`, `BinlocOrigin`, `BinlocDes`, `RtName`, `McName`, `ArticleID`, `ArticleName`, `StockQty`, `BaseUnit`, `Batch`, `Serial`, `MfgDate`, `ExpDate`, `ShelfLift`, `ReqNo`, `ReasonName`, `Remark`)
	for _, v := range rowReportItems.Columns {
		rowReportItems.ColumnTypes = append(rowReportItems.ColumnTypes, sqlx.ColumnType{
			Name:             v,
			DatabaseTypeName: "TEXT",
		})
	}

	return rowReportItems
}
