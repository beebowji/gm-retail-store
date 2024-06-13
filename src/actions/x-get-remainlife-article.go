package actions

import (
	"fmt"
	"time"

	"gitlab.dohome.technology/dohome-2020/gm-retail-store/src/common"
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tox"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
)

type _X_GET_REMAINLIFE_ARTICLE_RQB struct {
	ArticleId string     `json:"article_id"`
	MfgDate   *time.Time `json:"mfg_date"`
	ExpDate   *time.Time `json:"exp_date"`
}

type _X_GET_REMAINLIFE_ARTICLE_RSB struct {
	Remainlife   int        `json:"remainlife"`
	MfgDate      *time.Time `json:"mfg_date"`
	ExpDate      *time.Time `json:"exp_date"`
	IsRemainlift bool       `json:"is_remainlift"`
	Status       bool       `json:"status"`
	Msg          string     `json:"msg"`
}

func XGetRemainlifeArticle(c *gwx.Context) (any, error) {

	var dto _X_GET_REMAINLIFE_ARTICLE_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	if ex := c.Empty(dto.ArticleId, `กรุณาระบุ ArticleId`); ex != nil {
		return nil, ex
	}
	if dto.MfgDate == nil && dto.ExpDate == nil {
		return nil, c.Error("mfg_date and exp_date must be sent").StatusBadRequest()
	}

	// query
	query := fmt.Sprintf(`select tot_shelf_life from products where article_id = '%v'`, dto.ArticleId)
	row, ex := dbs.DH_ARTICLE_MASTER_R.QueryScan(query)
	if ex != nil {
		return nil, ex
	}

	// Check if tot_shelf_life is 0 or empty
	if row.Rows[0].Int(`tot_shelf_life`) == 0 || validx.IsEmpty(tox.String(row.Rows[0].Int(`tot_shelf_life`))) {
		return _X_GET_REMAINLIFE_ARTICLE_RSB{
			Remainlife:   0,
			MfgDate:      nil,
			ExpDate:      nil,
			IsRemainlift: false,
			Status:       false,
			Msg:          "สินค้านี้ไม่ใช่สินค้ามีอายุ",
		}, nil
	}

	// วันที่ปัจจุบัน
	today := time.Now()

	// Calculate remaining life
	response := calculateRemainingLife(dto, row, today)

	return response, nil

}

func calculateRemainingLife(dto _X_GET_REMAINLIFE_ARTICLE_RQB, row *sqlx.Rows, today time.Time) _X_GET_REMAINLIFE_ARTICLE_RSB {
	var mfg, exp *time.Time
	var remainingLife float64

	switch {
	case dto.MfgDate != nil:
		mfg = dto.MfgDate
		exp = common.AddDays(mfg, row.Rows[0].Int(`tot_shelf_life`))

		remainingLife = exp.Sub(today).Hours() / 24

	case dto.ExpDate != nil:
		exp = dto.ExpDate
		mfg = common.SubtractDays(exp, row.Rows[0].Int(`tot_shelf_life`))

		remainingLife = exp.Sub(today).Hours() / 24
	}

	return _X_GET_REMAINLIFE_ARTICLE_RSB{
		Remainlife:   tox.Int(remainingLife),
		MfgDate:      mfg,
		ExpDate:      exp,
		IsRemainlift: true,
		Status:       true,
		Msg:          "",
	}
}
