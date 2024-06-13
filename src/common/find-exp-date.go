package common

import (
	"fmt"
	"time"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tox"
)

func FindExpDate(articleId string, mfg *time.Time) *time.Time {

	// Connect
	dxArticleMaster, ex := sqlx.ConnectPostgresRW(dbs.DH_ARTICLE_MASTER)
	if ex != nil {
		return nil
	}

	query := fmt.Sprintf(`select tot_shelf_life from products where article_id = '%v'`, articleId)
	row, ex := dxArticleMaster.QueryScan(query)
	if ex != nil {
		return nil
	}

	result := mfg.AddDate(0, 0, tox.Int(row.Rows[0].Float(`tot_shelf_life`)))

	return &result

}
