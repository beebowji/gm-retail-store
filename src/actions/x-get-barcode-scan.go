package actions

import (
	"fmt"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
)

type X_ARTICLE_DISPLAY struct {
	Barcode   string `json:"barcode"`
	ArticleId string `json:"article_id"`
	UnitCode  string `json:"unit_code"`
	UnitName  string `json:"unit_name"`
}

func XGetBarcode(c *gwx.Context) (any, error) {
	var dto X_ARTICLE_DISPLAY
	dto.Barcode = c.Param("barcode")
	rsx, err := XArticleDisplay(dto, true)

	if err != nil {
		return nil, err
	}

	if rsx.ArticleId != "" {
		return rsx, nil
	}

	if rsx.ArticleId == "" {
		dto.ArticleId = c.Param("barcode")
		rsx, err = XArticleDisplay(dto, false)
		if err != nil {
			return rsx, err
		}
	}

	if rsx.ArticleId == "" {
		return rsx, fmt.Errorf("ไม่พบข้อมูลสินค้า")
	}

	return rsx, nil
}

func XArticleDisplay(dto X_ARTICLE_DISPLAY, isBarcode bool) (X_ARTICLE_DISPLAY, error) {

	query := `
			   SELECT
			   p.id,
			   p.article_id,
			   p.name_th as product_name,
			   u.unit_code,
			   u.name_th as unit_name,
			   pu.rate_unit_code,
			   pu.rate_unit_base, 
			   pb.barcode, 
			   pu.is_unit_base 
			  
		   FROM
			   products p
			   JOIN product_units pu ON p.id = pu.products_id
			   JOIN units u ON pu.units_id = u.id
			   LEFT JOIN purchaser_groups pg ON pg.group_no = p.purchaser_group_no
			   LEFT JOIN brands b ON p.brand_id = b.id 
			   LEFT JOIN product_barcodes pb ON pb.article_id = p.article_id and pb.unit_code = u.unit_code
		   WHERE 1=1 `

	if isBarcode {
		query += ` and pb.barcode = '` + dto.Barcode + `' `
	} else {
		query += ` and p.article_id = '` + dto.ArticleId + `' `
	}

	query += ` group by   
					p.id,
					p.article_id,
					p.name_th ,
					u.unit_code,
					u.name_th ,
					pu.rate_unit_code,
					pu.rate_unit_base, 
					pb.barcode, 
					pu.is_unit_base `

	rowsProducts, err := dbs.DH_ARTICLE_MASTER_R.QueryScan(query)
	if err != nil {
		return dto, err
	}
	dto.ArticleId = ""
	dto.Barcode = ""
	for _, v := range rowsProducts.Rows {
		dto.ArticleId = v.String(`article_id`)
		dto.Barcode = v.String("barcode")
		if isBarcode {
			dto.UnitCode = v.String("unit_code")
			dto.UnitName = v.String("unit_name")
			break
		} else if !isBarcode && v.Bool("is_unit_base") {
			dto.UnitCode = v.String("unit_code")
			dto.UnitName = v.String("unit_name")
			break
		}
	}

	return dto, nil
}
