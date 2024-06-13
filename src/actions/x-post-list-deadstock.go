package actions

import (
	"fmt"
	"strings"

	"gitlab.dohome.technology/dohome-2020/gm-retail-store/src/common"
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tablex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
	cuarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/cu-article-master"
)

type _X_POST_LIST_DEADSTOCK_RQB struct {
	SiteSloc []struct {
		Site string `json:"site"`
		Sloc string `json:"sloc"`
	} `json:"site_sloc"`
	Condition    string   `json:"condition"`
	Days         int64    `json:"days"`
	DaysDes      int64    `json:"days_des"`
	ArticleID    []string `json:"article_id"`
	McCode       []string `json:"mc_code"`
	RtCode       []string `json:"rt_code"`
	ReasonID     []string `json:"reason_id"`
	IsExportFile string   `json:"is_export_file"`
}

func XPostListDeadstock(c *gwx.Context) (any, error) {
	var dto _X_POST_LIST_DEADSTOCK_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	if err := validateRequest(dto, c); err != nil {
		return nil, err
	}

	// Connect
	dxRetailStore, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}

	// Construct SQL query
	query := constructDeadstockQuery(dto)

	rows, err := tablex.ExReport(c, dxRetailStore, query, ``)
	if err != nil {
		return nil, err
	}
	if len(rows.Rows) == 0 {
		return nil, nil
	}

	numRow := 1
	for _, v := range rows.Rows {
		// convert to unit_name
		baseUnitName := cuarticlemaster.UnitByKey(v.String(`base_unit`))
		v.Set(`base_unit`, baseUnitName.String(`name_th`))

		if v.TimePtr(`recv_date`) != nil {
			mfgLocal := v.TimePtr(`recv_date`).Local()
			v.Set(`recv_date`, mfgLocal.Format("2006-01-02"))
		}

		if v.TimePtr(`wd_date`) != nil {
			mfgLocal := v.TimePtr(`wd_date`).Local()
			v.Set(`wd_date`, mfgLocal.Format("2006-01-02"))
		}

		v.Set(`no`, numRow)
		numRow++
	}

	rowReportItems := sqlx.NewRows()
	rowReportItems.Rows = append(rowReportItems.Rows, rows.Rows...)
	rowReportItems.Columns = append(rowReportItems.Columns, `No`, `Site`, `Sloc`, `RtName`, `McName`, `ArticleID`, `ArticleName`, `Qty`, `BaseUnit`, `RecvDate`, `WdDate`, `DaysDeadstock`)
	for _, v := range rowReportItems.Columns {
		rowReportItems.ColumnTypes = append(rowReportItems.ColumnTypes, sqlx.ColumnType{
			Name:             v,
			DatabaseTypeName: "TEXT",
		})
	}

	if !validx.IsEmpty(dto.IsExportFile) {
		columns := []string{"ลำดับ", "site", "sloc", "ผู้ดูแลขาย", "หมวดหมู่สินค้า", "รหัสสินค้า", "ชื่อสินค้า", "จำนวน", "หน่วย", "วันที่นำเข้า", "วันที่เบิกออกล่าสุด", "จำนวนวันที่ไม่มีการเคลื่อนไหว"}
		excelURL, err := common.ExportExcelRpl("รายงานสินค้าที่ไม่มีการเคลื่อนไหวในคลังM", rowReportItems, columns)
		if err != nil {
			return nil, fmt.Errorf("failed to export data to Excel: %v", err)
		}

		return excelURL, nil
	}

	return rows, nil
}

func validateRequest(dto _X_POST_LIST_DEADSTOCK_RQB, c *gwx.Context) error {
	for _, v := range dto.SiteSloc {
		if err := c.Empty(v.Site, "กรุณาระบุ Site"); err != nil {
			return err
		}
		if err := c.Empty(v.Sloc, "กรุณาระบุ Slocs"); err != nil {
			return err
		}
	}
	return nil
}

func constructDeadstockQuery(dto _X_POST_LIST_DEADSTOCK_RQB) string {
	// Construct SQL query based on request parameters
	var siteSloc []string
	for _, v := range dto.SiteSloc {
		key := fmt.Sprintf(`('%v','%v')`, v.Site, v.Sloc)
		siteSloc = append(siteSloc, key)
	}
	conditions := make([]string, 0)
	conditions = append(conditions, `msw.stock_qty > 0`)
	conditions = append(conditions, fmt.Sprintf(`(ms.site,ms.sloc) in (%v)`, strings.Join(siteSloc, `,`)))
	if len(dto.ArticleID) > 0 {
		conditions = append(conditions, fmt.Sprintf(`msw.article_id in ('%v')`, strings.Join(dto.ArticleID, `','`)))
	}
	if len(dto.RtCode) > 0 {
		conditions = append(conditions, fmt.Sprintf(`sr.seller_code in ('%v')`, strings.Join(dto.RtCode, `','`)))
	}
	if len(dto.McCode) > 0 {
		conditions = append(conditions, fmt.Sprintf(`pc.mc_code in ('%v')`, strings.Join(dto.McCode, `','`)))
	}
	if len(dto.ReasonID) > 0 {
		conditions = append(conditions, fmt.Sprintf(`msw.wd_reason_id in ('%v')`, strings.Join(dto.ReasonID, `','`)))
	}

	baseQuery := `SELECT rt_name, mc_name, article_id, article_name, site, sloc, sum(stock_qty) as qty, base_unit, recv_date, wd_date, days_deadstock
	FROM (SELECT CONCAT(sr.seller_code, '  ', sr.seller_name) as rt_name,
	CONCAT(LEFT(pc.mc_code,3), '  ', pc.mc_name_th) as mc_name,
	ms.article_id,
	p.name_th as article_name,
	ms.stock_qty,
	ms.base_unit,
	ms.site, 
	ms.sloc,
	CASE
        WHEN MIN(msr.create_dtm) IS NULL THEN NULL
        WHEN MAX(msw.create_dtm) IS NULL THEN MIN(msr.create_dtm)
        ELSE NULL
    END AS recv_date,
	max(msw.create_dtm) as wd_date,
	CASE
        WHEN MIN(msr.create_dtm) IS NULL THEN EXTRACT(DAY FROM AGE(CURRENT_DATE, MAX(msw.create_dtm::date)))
        WHEN MAX(msw.create_dtm) IS NULL THEN EXTRACT(DAY FROM AGE(CURRENT_DATE, MIN(msr.create_dtm::date)))
        ELSE EXTRACT(DAY FROM AGE(CURRENT_DATE, MAX(msw.create_dtm::date)))
    END AS days_deadstock
	FROM m_stock ms
    LEFT JOIN m_stock_receive msr ON ms.article_id = msr.article_id
    LEFT JOIN m_stock_withdraw msw ON ms.article_id = msw.article_id
    LEFT JOIN dh_article_master.products p ON ms.article_id = p.article_id
    LEFT JOIN dh_article_master.product_categories pc ON pc.mc_code = p.merchandise_category2
    LEFT JOIN dh_article_master.sales_representative sr ON sr.id = p.zmm_seller
	WHERE ` + strings.Join(conditions, " AND ")

	if isOnlySiteSlocInDeadstock(dto) { // ถ้าส่งมาแค่ site sloc ดึงข้อมูลออกไปแสดงผลโชว์เป็นตัวอย่าง default
		baseQuery += ` GROUP BY ms.article_id, ms.site, ms.sloc, ms.stock_qty, p.name_th, ms.base_unit, rt_name, mc_name ORDER BY msr.create_dtm DESC LIMIT 10`
	} else {
		baseQuery += ` GROUP BY ms.article_id, ms.site, ms.sloc, ms.stock_qty, p.name_th, ms.base_unit, rt_name, mc_name`
	}

	if !validx.IsEmpty(dto.Condition) && dto.Condition != `between` {
		baseQuery += fmt.Sprintf(`) AS subquery WHERE days_deadstock %v %v`, dto.Condition, dto.Days)
	} else {
		baseQuery += fmt.Sprintf(`) AS subquery WHERE days_deadstock between %v and %v`, dto.Days, dto.DaysDes)
	}

	baseQuery += " group by rt_name, mc_name, article_id, article_name, site, sloc, base_unit, recv_date, wd_date, days_deadstock ORDER BY site ASC, days_deadstock DESC, mc_name ASC, rt_name ASC, article_id ASC"

	return fmt.Sprintf(`(%s) as t`, baseQuery)
}

// Function to check if only SiteSloc is provided and all other fields are null
func isOnlySiteSlocInDeadstock(dto _X_POST_LIST_DEADSTOCK_RQB) bool {
	return dto.DaysDes == 0 &&
		len(dto.ArticleID) == 0 &&
		len(dto.McCode) == 0 &&
		len(dto.RtCode) == 0 &&
		len(dto.ReasonID) == 0
}
