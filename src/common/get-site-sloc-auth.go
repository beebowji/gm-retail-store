package common

import (
	"fmt"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	gmauthen "gitlab.dohome.technology/dohome-2020/go-structx/gm-authen"
)

func GetSiteSlocAuth(userLogin *gmauthen.LoginResult, site, sloc string) (bool, error) {

	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_COMPANY)
	if ex != nil {
		return false, ex
	}

	// query site sloc
	query := fmt.Sprintf(`select * from site_slocs where site_code = '%v' and sloc_code = '%v'`, site, sloc)
	rows, ex := dx.QueryScan(query)
	if ex != nil {
		return false, ex
	}

	keySiteSloc := rows.Rows[0].String("site_code") + rows.Rows[0].String("sloc_code") + "web-retail-store"

	var chk bool
	for _, v := range userLogin.ModuleRoles {
		for _, l := range v.SiteSloc {
			if l.SiteCode+l.SlocCode+v.ModuleId == keySiteSloc {
				chk = true
			}
		}
	}

	return chk, ex
}
