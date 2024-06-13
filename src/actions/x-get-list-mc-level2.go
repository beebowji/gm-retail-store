package actions

import (
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
)

type _X_GET_LIST_MC_LEVEL2_RSB struct {
	McList []ItemMc `json:"mc_list"`
}

type ItemMc struct {
	McCode   string `json:"mc_code"`
	McNameTh string `json:"mc_name_th"`
}

func XGetListMcLevel2(c *gwx.Context) (any, error) {

	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_ARTICLE_MASTER)
	if ex != nil {
		return nil, ex
	}

	qry := `select mc_code, mc_name_th from product_categories where mc_code = LEFT(mc_code,3)`
	row, ex := dx.QueryScan(qry)
	if ex != nil {
		return nil, ex
	}

	var items []ItemMc
	for _, v := range row.Rows {
		items = append(items, ItemMc{
			McCode:   v.String(`mc_code`),
			McNameTh: v.String(`mc_name_th`),
		})
	}

	rto := _X_GET_LIST_MC_LEVEL2_RSB{
		McList: items,
	}

	return rto, nil

}
