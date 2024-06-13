package actions

import (
	"fmt"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
)

type _X_POST_ARTICLE_ISBATCH_RQB struct {
	ArticleId string `json:"article_id"`
	UnitCode  string `json:"unit_code"`
	SiteCode  string `json:"site_code"`
	SlocCode  string `json:"sloc_code"`
	BatchNo   string `json:"batch_no"`
}

type _X_POST_ARTICLE_ISBATCH_RSB struct {
	BatchNo   string `json:"batch_no"`
	IsBatchNo bool   `json:"is_batch_no"`
}

func XPostArticleIsbatch(c *gwx.Context) (any, error) {

	var dto _X_POST_ARTICLE_ISBATCH_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	// validate
	if ex := c.Empty(dto.ArticleId, `กรุณาระบุ ArticleId`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(dto.UnitCode, `กรุณาระบุ UnitCode`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(dto.SiteCode, `กรุณาระบุ SiteCode`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(dto.SlocCode, `กรุณาระบุ SlocCode`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(dto.BatchNo, `กรุณาระบุ BatchNo`); ex != nil {
		return nil, ex
	}

	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_COMMERCE)
	if ex != nil {
		return nil, ex
	}

	qry := fmt.Sprintf(`select article_id, batch_no from stock_batchs 
	where article_id = '%v' and unit_code = '%v' and site_code = '%v' and sloc_code = '%v' and batch_no = '%v'`, dto.ArticleId, dto.UnitCode, dto.SiteCode, dto.SlocCode, dto.BatchNo)
	row, ex := dx.QueryScan(qry)
	if ex != nil {
		return nil, ex
	}

	var isBatch bool
	if len(row.Rows) > 0 {
		isBatch = true
	}

	rto := _X_POST_ARTICLE_ISBATCH_RSB{
		BatchNo:   dto.ArticleId,
		IsBatchNo: isBatch,
	}

	return rto, nil

}
