package actions

import (
	"fmt"
	"sort"
	"strings"

	"gitlab.dohome.technology/dohome-2020/gm-retail-store/src/common"
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tablex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
)

type _X_POST_ARTICLE_INSTOCK_RQB struct {
	Site     string   `json:"site"`
	Sloc     string   `json:"sloc"`
	McCode   string   `json:"mc_code"`
	RtCode   string   `json:"rt_code"`
	Type     string   `json:"type"`
	CustomIn []string `json:"custom_in"`
}

func XPostArticleInstock(c *gwx.Context) (any, error) {
	var dto _X_POST_ARTICLE_INSTOCK_RQB
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

	var customIn []string
	for _, v := range dto.CustomIn {
		if strings.TrimSpace(v) == "" {
			continue
		}
		customIn = append(customIn, strings.TrimSpace(v))
	}
	dto.CustomIn = customIn
	// chk กรณีที่ส่งมาแค่ site กับ sloc
	qryByDtm := validx.IsEmpty(dto.McCode) && validx.IsEmpty(dto.RtCode) && validx.IsEmpty(dto.Type) && len(dto.CustomIn) == 0

	// Connect
	dxRetailStore, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}

	// Execute main query
	mainQuery := fmt.Sprintf(`(%s) as t`, buildMainQuery(dto, qryByDtm))
	datas, err := tablex.ExReport(c, dxRetailStore, mainQuery, ``)
	if err != nil {
		return nil, err
	}

	// Find unitName
	mapUnit, ex := common.FindUnitName()
	if ex != nil {
		return nil, ex
	}

	for _, v := range datas.Rows {
		// คำนวณอายุคงเหลือของสินค้า
		v.Set("shelf_lift", nil)
		if expDate := v.TimePtr("exp_date"); expDate != nil {
			v.Set("shelf_lift", common.CalculateDaysDiff(expDate))
		}

		// find unit name
		if findUnitName := mapUnit.FindMap(v.String("base_unit")); findUnitName != nil {
			v.Set("base_unit_name", findUnitName.String("name_th"))
		}
	}

	sort.Slice(datas.Rows, func(i, j int) bool {
		return datas.Rows[i].Int(`shelf_lift`) < datas.Rows[j].Int(`shelf_lift`)
	})

	return datas, nil
}

// Function to build the main query based on the request parameters
func buildMainQuery(dto _X_POST_ARTICLE_INSTOCK_RQB, qryByDtm bool) string {
	// Construct the main query based on dto
	qryMStock := fmt.Sprintf(`select ms.id
	, p.article_id
	, p.name_th as article_name
	, batch
	, serial
	, mfg_date
	, exp_date
	, bin_location
	, site
	, sloc
	, stock_qty
	, base_unit
	, create_dtm
	, update_dtm
	, update_by
	, ms.remark
    from m_stock ms
    left join dh_article_master.products p on ms.article_id = p.article_id
    left join dh_article_master.sales_representative sr on p.zmm_seller = sr.id 
    left join dh_article_master.product_categories pc on p.merchandise_category2 = pc.mc_code 
    left join dh_article_master.product_barcodes pb on pb.article_id = p.article_id 
    where site = '%v' and sloc = '%v' and stock_qty <> '0'`, dto.Site, dto.Sloc)
	// Add conditions based on 'McCode' and 'RtCode'
	if !validx.IsEmpty(dto.McCode) {
		qryMStock += fmt.Sprintf(` and pc.mc_code = '%v'`, dto.McCode)
	}
	if !validx.IsEmpty(dto.RtCode) {
		qryMStock += fmt.Sprintf(` and sr.seller_code = '%v'`, dto.RtCode)
	}
	// Add the condition based on the 'type' field
	if len(dto.CustomIn) > 0 {
		switch dto.Type {
		case "article_id":
			qryMStock += fmt.Sprintf(` and p.article_id in ('%v')`, strings.Join(dto.CustomIn, `','`))
		case "barcode":
			qryMStock += fmt.Sprintf(` and pb.barcode in ('%v')`, strings.Join(dto.CustomIn, `','`))
		case "article_name":
			var conditions []string
			for _, pattern := range dto.CustomIn {
				pattern = strings.ToLower(pattern) // Convert pattern to lowercase
				conditions = append(conditions, fmt.Sprintf("LOWER(p.name_th) LIKE '%%%s%%'", pattern))
			}
			qryMStock += " AND (" + strings.Join(conditions, " or ") + ")"
		case "serial":
			qryMStock += fmt.Sprintf(` and serial in ('%v')`, strings.Join(dto.CustomIn, `','`))
		case "batch":
			qryMStock += fmt.Sprintf(` and batch in ('%v')`, strings.Join(dto.CustomIn, `','`))
		case "binloc":
			qryMStock += fmt.Sprintf(` and bin_location in ('%v')`, strings.Join(dto.CustomIn, `','`))
		}
	}

	if qryByDtm {
		qryMStock += " order by create_dtm desc limit 10"
	}
	qryMStock += " group by ms.id, p.article_id, p.name_th, batch, serial, mfg_date, exp_date, bin_location, site, sloc, stock_qty, base_unit, create_dtm, update_dtm, update_by, ms.remark ORDER BY p.article_id, bin_location, serial, batch"

	return qryMStock
}
