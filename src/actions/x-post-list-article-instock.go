package actions

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"gitlab.dohome.technology/dohome-2020/gm-retail-store/src/common"
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tablex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
	cuarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/cu-article-master"
)

type _X_POST_LIST_ARTICLE_INSTOCK_RQB struct {
	SiteSloc []struct {
		Site string `json:"site"`
		Sloc string `json:"sloc"`
	} `json:"site_sloc"`
	ArticleID    []string    `json:"article_id"`
	Binloc       interface{} `json:"binloc"`
	Binlocs      []string    `json:"binlocs"`
	Condition    string      `json:"condition"`
	ShelfLife    int64       `json:"shelf_life"`
	ShelfLiftEnd int64       `json:"avg_end"`
	Serial       string      `json:"serial"`
	Batch        string      `json:"batch"`
	McCode       []string    `json:"mc_code"`
	RtCode       []string    `json:"rt_code"`
	ReasonID     []string    `json:"reason_id"`
	IsExportFile string      `json:"is_export_file"`
}

func XPostListArticleInstock(c *gwx.Context) (any, error) {
	var dto _X_POST_LIST_ARTICLE_INSTOCK_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	binlocX := fmt.Sprintf("%v", dto.Binloc)

	if binlocX != "" {
		binlocX = strings.Replace(binlocX, "[", "", 100)
		binlocX = strings.Replace(binlocX, "]", "", 100)
		if binlocX != "" {
			if strings.Contains(binlocX, ",") {
				dto.Binlocs = strings.Split(binlocX, ",")
			} else {
				dto.Binlocs = strings.Split(binlocX, " ")
			}
		}
	} else {
		dto.Binlocs = []string{}
	}

	// Validate input
	if err := validateInputInstock(&dto); err != nil {
		return nil, err
	}

	if len(dto.Binlocs) > 20 {
		return nil, c.Error(`Binloc เกิน 20`).StatusBadRequest()
	}

	// Connect
	dxRetailStore, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}

	var siteSloc []string
	for _, v := range dto.SiteSloc {
		siteSloc = append(siteSloc, fmt.Sprintf("('%v','%v')", v.Site, v.Sloc))
	}

	query := fmt.Sprintf(`select ms.bin_location,
	ms.site,
	ms.sloc,
	ms.article_id,
	p.name_th as article_name,
	ms.stock_qty,
	ms.base_unit,
	ms.batch,
	ms.serial,
	concat(sr.seller_code , ' ', sr.seller_name) as rt_name,
	concat(p.merchandise_category2  , ' ' , pc.mc_name_th ) as mc_name,
	ms.exp_date,
	ms.remark
	FROM m_stock ms           
	left join dh_article_master.products p on ms.article_id = p.article_id         
	left join dh_article_master.product_categories pc on pc.mc_code = p.merchandise_category2           
	left join dh_article_master.sales_representative sr on sr.id = p.zmm_seller
	where ms.stock_qty > 0 and (ms.site,ms.sloc) in (%v)`, strings.Join(siteSloc, `,`))

	if len(dto.ArticleID) > 0 {
		query += fmt.Sprintf(` and ms.article_id in ('%v')`, strings.Join(dto.ArticleID, `','`))
	}
	if len(dto.Binlocs) > 0 {
		query += fmt.Sprintf(` and ms.bin_location in ('%v')`, strings.Join(dto.Binlocs, `','`))
	}
	if !validx.IsEmpty(dto.Serial) {
		query += fmt.Sprintf(` and ms.serial = '%v'`, dto.Serial)
	}
	if !validx.IsEmpty(dto.Batch) {
		query += fmt.Sprintf(` and ms.batch = '%v'`, dto.Batch)
	}
	if len(dto.McCode) > 0 {
		query += fmt.Sprintf(` and pc.mc_code in ('%v')`, strings.Join(dto.McCode, `','`))
	}
	if len(dto.RtCode) > 0 {
		query += fmt.Sprintf(` and sr.seller_code in ('%v')`, strings.Join(dto.RtCode, `','`))
	}

	if !validx.IsEmpty(dto.Condition) && dto.Condition != "between" {
		query += fmt.Sprintf(` and exp_date is not null and ((ms.exp_date at time zone 'Asia/Bangkok')::timestamp - CURRENT_DATE) %v INTERVAL '%v day'`, dto.Condition, dto.ShelfLife)
	} else if dto.Condition == "between" {
		query += fmt.Sprintf(` and exp_date is not null and ((ms.exp_date at time zone 'Asia/Bangkok')::timestamp - CURRENT_DATE) BETWEEN INTERVAL '%v day' and INTERVAL '%v day'`, dto.ShelfLife, dto.ShelfLiftEnd)
	}

	query += ` order by site, article_id, bin_location, serial, batch`

	mainQuery := fmt.Sprintf(`(%s) as t`, query)
	rows, err := tablex.ExReport(c, dxRetailStore, mainQuery, ``)
	if err != nil {
		return nil, err
	}
	if len(rows.Rows) == 0 {
		return nil, nil
	}

	numRow := 1
	for _, v := range rows.Rows {
		// convert to unit_name
		v.Set(`base_unit`, cuarticlemaster.UnitByKey(v.String(`base_unit`)).String(`name_th`))

		// find ShelfLift
		if v.TimePtr(`exp_date`) != nil {
			v.Set(`shelf_life`, common.CalculateDaysDiff(v.TimePtr(`exp_date`)))

			mfgLocal := v.TimePtr(`exp_date`).Local()
			v.Set(`exp_date`, mfgLocal.Format("2006-01-02"))
		} else {
			v.Set(`shelf_life`, "")
		}

		v.Set(`no`, numRow)
		numRow++
	}

	rowReportItems := sqlx.NewRows()
	rowReportItems.Rows = append(rowReportItems.Rows, rows.Rows...)
	rowReportItems.Columns = append(rowReportItems.Columns, `No`, `Site`, `Sloc`, `BinLocation`, `ArticleID`, `ArticleName`, `StockQty`, `BaseUnit`, `Batch`, `Serial`, `ShelfLife`, `RtName`, `McName`, `Remark`)
	for _, v := range rowReportItems.Columns {
		rowReportItems.ColumnTypes = append(rowReportItems.ColumnTypes, sqlx.ColumnType{
			Name:             v,
			DatabaseTypeName: "TEXT",
		})
	}

	if !validx.IsEmpty(dto.IsExportFile) {
		columns := []string{"ลำดับ", "site", "sloc", "ตำแหน่ง", "รหัสสินค้า", "ชื่อสินค้า", "จำนวน", "หน่วย", "Batch", "Serial", "อายุคงเหลือสินค้า(วัน)", "ผู้ดูแลขาย", "หมวดหมู่สินค้า", "หมายเหตุ"}
		excelURL, err := common.ExportExcelRpl("รายงานสินค้าที่จัดเก็บในคลังMค้าปลีก", rowReportItems, columns)
		if err != nil {
			return nil, fmt.Errorf("failed to export data to Excel: %v", err)
		}
		return excelURL, nil
	}

	sort.Slice(rows.Rows, func(i, j int) bool {
		if rows.Rows[i].Int(`shelf_life`) == rows.Rows[j].Int(`shelf_life`) {
			return rows.Rows[i].Int(`rt_name`) < rows.Rows[j].Int(`rt_name`)
		}
		return rows.Rows[i].Int(`shelf_life`) < rows.Rows[j].Int(`shelf_life`)
	})

	return rows, nil
}

func validateInputInstock(dto *_X_POST_LIST_ARTICLE_INSTOCK_RQB) error {
	if len(dto.SiteSloc) == 0 {
		return errors.New("site_sloc is required")
	}

	for _, v := range dto.SiteSloc {
		if v.Site == "" {
			return errors.New("site is required")
		}
		if v.Sloc == "" {
			return errors.New("sloc is required")
		}
	}
	return nil
}
