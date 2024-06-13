package actions

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"gitlab.dohome.technology/dohome-2020/gm-retail-store/src/common"
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/logx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/timex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tox"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
	cuarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/cu-article-master"
)

type _X_POST_LIST_WD_OFTEN_RQB struct {
	SiteSloc     []SiteSloc `json:"site_sloc"`
	BeginDate    *time.Time `json:"begin_date"`
	EndDate      *time.Time `json:"end_date"`
	Condition    string     `json:"condition"`
	Avg          int        `json:"avg"`
	AvgEnd       int        `json:"avg_end"`
	ArticleID    []string   `json:"article_id"`
	ReasonID     []string   `json:"reason_id"`
	McCode       []string   `json:"mc_code"`
	RtCode       []string   `json:"rt_code"`
	IsExportFile string     `json:"is_export_file"`
}

type SiteSloc struct {
	Site string `json:"site"`
	Sloc string `json:"sloc"`
}

type Report struct {
	Rows       []sqlx.Map `json:"rows"`
	TotalCount int        `json:"totalCount"`
}

func XPostListWdOften(c *gwx.Context) (any, error) {
	skip := c.Query(`skip`)
	take := c.Query(`take`)
	ttCount := c.Query(`requireTotalCount`)

	var dto _X_POST_LIST_WD_OFTEN_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	if err := validateDTO(dto, c); err != nil {
		return nil, err
	}

	siteSloc := buildSiteSloc(dto.SiteSloc)

	var sql strings.Builder
	sql.WriteString(buildSimpleQuery(siteSloc))
	if len(dto.ArticleID) != 0 || len(dto.ReasonID) != 0 || len(dto.McCode) != 0 || len(dto.RtCode) != 0 || dto.BeginDate != nil {
		sql.WriteString(buildWhereCondition(dto))
	}

	queryString := fmt.Sprintf("%s GROUP BY rt_name, mc_name, msw.article_id, p.name_th, ms.base_unit, msw.site, msw.sloc, msw.base_unit", sql.String())
	rows, err := dbs.DH_RETAIL_STORE_R.QueryScan(queryString)
	if err != nil {
		return nil, err
	}

	data, ex := SetDataStockWithdraw(rows.Rows, siteSloc)
	if ex != nil {
		return nil, ex
	}

	rowReportItems := sqlx.NewRows()
	numRow := 1
	for _, v := range data {
		avgWd := v.Float("avg_wd")

		// convert to unit_name
		baseUnitName := cuarticlemaster.UnitByKey(v.String(`base_unit`))
		v.Set(`base_unit`, baseUnitName.String(`name_th`))

		if dto.AvgEnd != 0 {
			if avgWd >= tox.Float(dto.Avg) && avgWd <= tox.Float(dto.AvgEnd) {
				rowReportItems.Rows = append(rowReportItems.Rows, createRowMap(v, numRow))
				numRow++
			}
		} else {
			switch dto.Condition {
			case "<":
				if avgWd < tox.Float(dto.Avg) {
					rowReportItems.Rows = append(rowReportItems.Rows, createRowMap(v, numRow))
					numRow++
				}
			case ">":
				if avgWd > tox.Float(dto.Avg) {
					rowReportItems.Rows = append(rowReportItems.Rows, createRowMap(v, numRow))
					numRow++
				}
			case "=":
				if avgWd == tox.Float(dto.Avg) {
					rowReportItems.Rows = append(rowReportItems.Rows, createRowMap(v, numRow))
					numRow++
				}
			}
		}
	}

	if !validx.IsEmpty(dto.IsExportFile) {
		rowReportItems.Columns = append(rowReportItems.Columns, "NO", "Site", "Sloc", "RtName", "McName", "ArticleId", "ArticleName", "StockQty", "BaseUnit", "SumAllWd", "SumQtyWd", "AvgWd", "AvgQtyWd")
		for _, v := range rowReportItems.Columns {
			rowReportItems.ColumnTypes = append(rowReportItems.ColumnTypes, sqlx.ColumnType{
				Name:             v,
				DatabaseTypeName: "TEXT",
			})
		}

		columns := []string{"ลำดับ", "site", "sloc", "ผู้ดูแลขาย", "หมวดหมู่สินค้า", "รหัสสินค้า", "ชื่อสินค้า", "จำนวนคงเหลือ", "หน่วย", "จำนวนครั้งที่เบิก", "จำนวนสินค้าที่เบิก", "% การเบิกสินค้าต่อครั้ง", "% จำนวนสินค้าที่เบิก"}
		excelURL, err := common.ExportExcelRpl("รายงานสินค้าเบิกบ่อย", rowReportItems, columns)
		if err != nil {
			return nil, fmt.Errorf("failed to export data to Excel: %v", err)
		}

		return excelURL, nil
	} else {
		// Calculate TotalCount based on filtered rows if required
		var totalCount int
		if ttCount == `true` {
			totalCount = len(rowReportItems.Rows)
		}

		// Apply pagination
		startIdx := tox.Int(skip)
		endIdx := startIdx + tox.Int(take)

		// Ensure end index is within range
		if endIdx > len(rowReportItems.Rows) {
			endIdx = len(rowReportItems.Rows)
		}

		// Slice the rows to the desired range
		paginatedRows := rowReportItems.Rows[startIdx:endIdx]

		report := Report{
			Rows:       paginatedRows,
			TotalCount: totalCount,
		}

		return report, nil
	}
}

func validateDTO(dto _X_POST_LIST_WD_OFTEN_RQB, c *gwx.Context) error {
	if dto.BeginDate == nil || dto.BeginDate.IsZero() {
		return errors.New("กรุณาระบุ begin_date")
	}
	if err := c.Empty(dto.Condition, "กรุณาระบุ condition"); err != nil {
		return err
	}
	if len(dto.SiteSloc) == 0 {
		return errors.New("กรุณาระบุ site_sloc อย่างน้อย 1 site_sloc")
	}
	for _, vSiteSloc := range dto.SiteSloc {
		if err := c.Empty(vSiteSloc.Site, "กรุณาระบุ site"); err != nil {
			return err
		}
		if err := c.Empty(vSiteSloc.Sloc, "กรุณาระบุ sloc"); err != nil {
			return err
		}
	}
	return nil
}

func buildSiteSloc(siteSlocs []SiteSloc) []string {
	result := make([]string, len(siteSlocs))
	for i, v := range siteSlocs {
		result[i] = fmt.Sprintf("('%v','%v')", v.Site, v.Sloc)
	}
	return result
}

func buildSimpleQuery(siteSloc []string) string {
	return fmt.Sprintf(`SELECT CONCAT(sr.seller_code, '  ' ,sr.seller_name) as rt_name,
	CONCAT(LEFT(pc.mc_code,3), '  ' ,pc.mc_name_th) as mc_name,
    msw.article_id AS article_id,
    p.name_th AS article_name,
    msw.base_unit AS base_unit,
    msw.site,
    msw.sloc,
    SUM(ms.stock_qty) AS stock_balance,
    COUNT(DISTINCT CONCAT(msw.req_no_wd, msw.article_id)) AS count_wd,
    SUM(msw.stock_qty) AS wd_qty
	FROM m_stock_withdraw msw  
	left JOIN m_stock ms ON msw.article_id = ms.article_id  
	left JOIN dh_article_master.products p ON p.article_id = msw.article_id
	left JOIN dh_article_master.product_categories pc ON pc.mc_code = p.merchandise_category2
	left JOIN dh_article_master.sales_representative sr ON sr.id = p.zmm_seller
    WHERE (msw.site,msw.sloc) IN (%v) `, strings.Join(siteSloc, ","))
}

func buildWhereCondition(dto _X_POST_LIST_WD_OFTEN_RQB) string {
	var conditions []string

	if len(dto.ArticleID) > 0 {
		conditions = append(conditions, fmt.Sprintf(`msw.article_id IN ('%s')`, strings.Join(dto.ArticleID, "','")))
	}
	if len(dto.ReasonID) > 0 {
		conditions = append(conditions, fmt.Sprintf(`msw.wd_reason_id IN ('%s')`, strings.Join(dto.ReasonID, "','")))
	}
	if len(dto.McCode) > 0 {
		conditions = append(conditions, fmt.Sprintf(`pc.mc_code IN ('%s')`, strings.Join(dto.McCode, "','")))
	}
	if len(dto.RtCode) > 0 {
		conditions = append(conditions, fmt.Sprintf(`sr.seller_code IN ('%s')`, strings.Join(dto.RtCode, "','")))
	}
	if dto.EndDate == nil || dto.EndDate.IsZero() {
		conditions = append(conditions, fmt.Sprintf(`msw.create_dtm::date = '%s'`, dto.BeginDate.Local().Format(timex.YYYYMMDD)))
	} else {
		conditions = append(conditions, fmt.Sprintf(`(msw.create_dtm::date BETWEEN '%s' AND '%s')`, dto.BeginDate.Local().Format(timex.YYYYMMDD), dto.EndDate.Local().Format(timex.YYYYMMDD)))
	}

	return "AND " + strings.Join(conditions, " AND ")
}

func SetDataStockWithdraw(rows []sqlx.Map, siteSloc []string) ([]sqlx.Map, error) {
	var articleIDs []string
	for _, row := range rows {
		articleIDs = append(articleIDs, row.String(`article_id`))
	}

	// Fetch last create_dtm, sum_all_wd, and sum_qty_wd for all rows in a single query
	sql := fmt.Sprintf(`SELECT msw.site,
    msw.sloc,
	msw.create_dtm,
    COUNT(DISTINCT CONCAT(msw.req_no_wd, msw.article_id)) AS sum_all_wd,
    SUM(ms.stock_qty) AS sum_qty_wd
	FROM m_stock_withdraw msw  
	left JOIN m_stock ms ON msw.article_id = ms.article_id  
	left JOIN dh_article_master.products p ON p.article_id = msw.article_id
	left JOIN dh_article_master.product_categories pc ON pc.mc_code = p.merchandise_category2
	left JOIN dh_article_master.sales_representative sr ON sr.id = p.zmm_seller
	WHERE msw.article_id IN ('%v')
	GROUP by msw.site, msw.sloc, msw.create_dtm`, strings.Join(articleIDs, `','`))
	rowsData, ex := dbs.DH_RETAIL_STORE_R.QueryScan(sql)
	if ex != nil {
		return nil, ex
	}

	// Iterate through rows and set data
	for _, row := range rows {
		data := rowsData.FindRow(func(m *sqlx.Map) bool {
			return m.String(`site`) == row.String(`site`) && m.String(`sloc`) == row.String(`sloc`)
		})

		logx.Infof(tox.String(row.Float(`stock_balance`)))

		if data != nil {
			row.Set("create_dtm", data.TimePtr("create_dtm"))

			// Calculate avg_wd and avg_sum_article_all with rounding to 2 decimal places
			avgWd := math.Round((row.Float(`count_wd`)/data.Float("sum_all_wd"))*100*100) / 100 // จำนวนครั้งที่เบิก / จำนวนครั้งที่เบิกในระบบทั้งหมด * 100
			if math.IsInf(avgWd, 0) || math.IsNaN(avgWd) {
				avgWd = 0
			}
			row.Set("avg_wd", avgWd)

			avgSumArticleAll := math.Round((row.Float(`wd_qty`)/data.Float("sum_qty_wd"))*100*100) / 100 // จำนวนชิ้นสินค้าที่เบิก / จำนวนชิ้นสินค้าที่เบิกในระบบทั้งหมด * 100
			if math.IsInf(avgSumArticleAll, 0) || math.IsNaN(avgSumArticleAll) {
				avgSumArticleAll = 0
			}
			row.Set("avg_sum_article_all", avgSumArticleAll)
		}
	}

	// Sort rows by create_dtm
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Time("create_dtm").After(rows[j].Time("create_dtm"))
	})

	return rows, nil
}

func createRowMap(v sqlx.Map, numRow int) sqlx.Map {
	stockQty := v.Float("stock_balance")
	stockQtyRounded := math.Round(stockQty*100) / 100         // ปัดทศนิยมให้มีแค่ 2 ตำแหน่ง
	stockQtyFormatted := fmt.Sprintf("%.2f", stockQtyRounded) // จัดรูปแบบเป็นทศนิยม 2 ตำแหน่ง

	return sqlx.Map{
		"no":           numRow,
		"site":         v.String("site"),
		"sloc":         v.String("sloc"),
		"rt_name":      v.String("rt_name"),
		"mc_name":      v.String("mc_name"),
		"article_id":   v.String("article_id"),
		"article_name": v.String("article_name"),
		"stock_qty":    stockQtyFormatted,
		"base_unit":    v.String("base_unit"),
		"sum_all_wd":   v.Float("count_wd"),
		"sum_qty_wd":   v.Float("wd_qty"),
		"avg_wd":       v.Float("avg_wd"),
		"avg_qty_wd":   v.Float("avg_sum_article_all"),
	}
}
