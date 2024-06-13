package actions

import (
	"fmt"
	"strings"
	"time"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tablex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/timex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
)

type X_POST_LIST_STATE_BINLOCATION_RQB struct {
	Site        string     `json:"site" form:"site"`
	Sloc        string     `json:"sloc" form:"sloc"`
	BinlocBegin []string   `json:"binloc_begin" form:"binloc_begin"`
	BinlocEnd   string     `json:"binloc_end" form:"binloc_end"`
	DateBegin   *time.Time `json:"date_begin" form:"date_begin"`
	DateEnd     *time.Time `json:"date_end" form:"date_end"`
	Status      string     `json:"status" form:"status"`
}

func XPostListStateBinlocation(c *gwx.Context) (any, error) {

	var dto X_POST_LIST_STATE_BINLOCATION_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	// Validate
	if validx.IsEmpty(dto.Site) {
		return nil, c.Error("กรุณาระบุ Site").StatusBadRequest()
	}
	if validx.IsEmpty(dto.Sloc) {
		return nil, c.Error("กรุณาระบุ Sloc").StatusBadRequest()
	}
	if len(dto.BinlocBegin) == 0 && dto.DateBegin == nil {
		return nil, c.Error("กรุณาระบุ BinlocBegin หรือ DateBegin").StatusBadRequest()
	}
	if len(dto.BinlocBegin) > 20 {
		return nil, c.Error(`BinlocBegin เกิน 20`).StatusBadRequest()
	}

	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_COMPANY)
	if ex != nil {
		return nil, ex
	}

	// Construct the SQL query
	qry := constructQuery(dto)

	// Execute the query
	mainQuery := fmt.Sprintf(`(%s) as t`, qry)
	rows, err := tablex.ExReport(c, dx, mainQuery, ``)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func constructQuery(dto X_POST_LIST_STATE_BINLOCATION_RQB) string {

	qry := fmt.Sprintf(`select binloc,
	perid,
	concat (pe.first_name,' ',pe.last_name) as person_name,
	createdate,
	createtime,
	aprvflag
	from bin_location_master bls
	left join employees pe on pe.person_id = usercreate
	where werks = '%s' and lgort = '%s'`, dto.Site, dto.Sloc)

	// where binloc
	if validx.IsEmpty(dto.BinlocEnd) && len(dto.BinlocBegin) != 0 {
		qry += fmt.Sprintf(` and bls.binloc in ('%v')`, strings.Join(dto.BinlocBegin, `','`))
	} else if !validx.IsEmpty(dto.BinlocEnd) && len(dto.BinlocBegin) == 1 {
		qry += fmt.Sprintf(` and bls.binloc between '%v' and '%v'`, dto.BinlocBegin[0], dto.BinlocEnd)
	}

	// where date
	if dto.DateBegin != nil && dto.DateEnd == nil {
		qry += fmt.Sprintf(` and createdate = '%v'`, dto.DateBegin.Local().Format(timex.YYYYMMDD))
	} else if dto.DateBegin != nil && dto.DateEnd != nil {
		qry += fmt.Sprintf(` and createdate between '%v' and '%v'`, dto.DateBegin.Local().Format(timex.YYYYMMDD), dto.DateEnd.Local().Format(timex.YYYYMMDD))
	}

	// where status
	if strings.ToUpper(dto.Status) != `ALL` {
		qry += fmt.Sprintf(` and bls.aprvflag = '%v'`, dto.Status)
	}

	return qry
}
