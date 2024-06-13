package common

import (
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
)

func FindUnitName() (*sqlx.Rows, error) {
	dxArticle, ex := sqlx.ConnectPostgresRO(dbs.DH_ARTICLE_MASTER)
	if ex != nil {
		return nil, ex
	}

	rows, ex := dxArticle.QueryScan(`select unit_code,name_th from units`)
	if ex != nil {
		return nil, ex
	}
	mapUnit := rows.BuildMap(func(m *sqlx.Map) string {
		return m.String(`unit_code`)
	})

	return mapUnit, nil
}
