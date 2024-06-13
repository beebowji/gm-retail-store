package actions

import (
	"fmt"

	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
	cucompany "gitlab.dohome.technology/dohome-2020/go-structx/cu-company"
	curetailstore "gitlab.dohome.technology/dohome-2020/go-structx/cu-retail-store"
)

func XGetHistoryRecv(c *gwx.Context) (any, error) {

	site := c.Query(`site`)
	sloc := c.Query(`sloc`)
	articleId := c.Query(`article_id`)
	binLocation := c.Query(`bin_location`)

	//validate
	if ex := c.Empty(site, `กรุณาระบุ site`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(sloc, `กรุณาระบุ sloc`); ex != nil {
		return nil, ex
	}
	siteSlocAll, ex := cucompany.SiteSlocs()
	if ex != nil {
		return nil, ex
	}

	resp := sqlx.NewRows()
	if !validx.IsEmpty(articleId) && validx.IsEmpty(binLocation) {
		sql := fmt.Sprintf(`
		( SELECT
        	msr.create_dtm,   
        	msr.req_no_recv,
        	'นำเข้า' as type,  
        	msr.article_id,
        	msr.batch,
        	msr.serial,
        	msr.mfg_date,
        	msr.exp_date,
        	msr.bin_location,
			msr.recv_qty,
        	msr.recv_unit
		FROM m_stock_receive msr
		WHERE msr.site = '%v' 
		AND msr.sloc = '%v' 
		AND msr.article_id = '%v' order by create_dtm desc LIMIT 3) as t`, site, sloc, articleId)
		rows, ex := curetailstore.M_STOCK_RECEIVE.ExRead(c, sql, `create_dtm desc `)
		if ex != nil {
			return nil, ex
		}
		for _, v := range rows.Rows {
			key := fmt.Sprintf(`%v|%v`, site, sloc)
			data := siteSlocAll.FindMap(key)
			if data != nil {
				siteName := fmt.Sprintf(`%v : %v`, site, data.String(`site_name`))
				v.Set(`site`, siteName)
			}
		}
		resp.Rows = append(resp.Rows, rows.Rows...)
	} else if !validx.IsEmpty(binLocation) && validx.IsEmpty(articleId) {
		sql := fmt.Sprintf(`(SELECT
			msr.create_dtm,   
			msr.req_no_recv,
			'นำเข้า' as type,  
			msr.article_id,
			msr.batch,
			msr.serial,
			msr.mfg_date,
			msr.exp_date,
			msr.bin_location,
			msr.recv_qty,
			msr.recv_unit
		FROM m_stock_receive msr
		WHERE msr.site = '%v' 
		AND msr.sloc = '%v' 
		AND msr.bin_location = '%v' order by create_dtm desc LIMIT 3) as t`, site, sloc, binLocation)
		rows, ex := curetailstore.M_STOCK_RECEIVE.ExRead(c, sql, `create_dtm desc `)
		if ex != nil {
			return nil, ex
		}
		for _, v := range rows.Rows {
			key := fmt.Sprintf(`%v|%v`, site, sloc)
			data := siteSlocAll.FindMap(key)
			if data != nil {
				siteName := fmt.Sprintf(`%v : %v`, site, data.String(`site_name`))
				v.Set(`site`, siteName)
			}
		}
		resp.Rows = append(resp.Rows, rows.Rows...)
	}
	return resp, nil
}
