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
	cuarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/cu-article-master"
)

type _X_POST_LIST_REPORT_REQ_RQB struct {
	SiteSloc []struct {
		Site string `json:"site"`
		Sloc string `json:"sloc"`
	} `json:"site_sloc"`
	BeginDate *time.Time `json:"begin_date"`
	EndDate   *time.Time `json:"end_date"`
	Req       []struct {
		ReqNo string `json:"req_no"`
	} `json:"req"`
	TypeReport string `json:"type_report"`
}

type _X_POST_LIST_REPORT_REQ_RSB struct {
	ReportList []ReportList `json:"report_list"`
	TotalCount int          `json:"totalCount"`
	Summary    []string     `json:"summary"`
}

type ReportList struct {
	ReqNo        string         `json:"req_no"`
	CreateDtm    string         `json:"create_dtm"`
	ReportDetail []ReportDetail `json:"report_detail"`
}

type ReportDetail struct {
	SeqNo      int     `json:"seq_no"`
	Site       string  `json:"site"`
	Sloc       string  `json:"sloc"`
	RtName     string  `json:"rt_name"`
	McName     string  `json:"mc_name"`
	ArticleID  string  `json:"article_id"`
	ArtcleName string  `json:"artcle_name"`
	Batch      string  `json:"batch"`
	Serial     string  `json:"serial"`
	MfgDate    string  `json:"mfg_date"`
	ExpDate    string  `json:"exp_date"`
	StockQty   float64 `json:"stock_qty"`
	BaseUnit   string  `json:"base_unit"`
	Binloc     string  `json:"binloc"`
}

func XPostListReportReq(c *gwx.Context) (any, error) {

	var dto _X_POST_LIST_REPORT_REQ_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	// Validate
	if len(dto.SiteSloc) > 0 {
		for _, v := range dto.SiteSloc {
			if ex := c.Empty(v.Site, `กรุณาระบุ Site`); ex != nil {
				return nil, ex
			}
			if ex := c.Empty(v.Sloc, `กรุณาระบุ Sloc`); ex != nil {
				return nil, ex
			}
		}
	} else {
		return nil, fmt.Errorf("กรุณาระบุ site sloc")
	}
	if ex := c.Empty(dto.TypeReport, `กรุณาระบุ TypeReport`); ex != nil {
		return nil, ex
	}

	// chk req no
	var isReqNo bool
	var reqNo []string
	for _, v := range dto.Req {
		if !validx.IsEmpty(v.ReqNo) {
			isReqNo = true
			reqNo = append(reqNo, v.ReqNo)
		}
	}

	if dto.BeginDate == nil && !isReqNo {
		return nil, fmt.Errorf("กรุณาระบุ BeginDate หรือ ReqNo")
	}
	if dto.BeginDate == nil && dto.EndDate != nil {
		return nil, fmt.Errorf("กรุณาระบุ BeginDate ด้วย")
	}

	// Connect
	dxRetailStore, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}

	// Convert TypeReport to lowercase
	dto.TypeReport = strings.ToLower(dto.TypeReport)

	qry, err := buildQuery(dto, reqNo)
	if err != nil {
		return nil, err
	}

	mainQuery := fmt.Sprintf(`(%s) as t`, qry)
	rows, err := tablex.ExReport(c, dxRetailStore, mainQuery, ``)
	if err != nil {
		return nil, err
	}

	// Process query results
	reportList := processQueryResults(rows.Rows)

	rto := _X_POST_LIST_REPORT_REQ_RSB{
		ReportList: reportList,
		TotalCount: int(rows.TotalCount),
	}

	return rto, nil

}

func buildQuery(req _X_POST_LIST_REPORT_REQ_RQB, reqNo []string) (string, error) {
	var siteSloc []string
	for _, v := range req.SiteSloc {
		key := fmt.Sprintf(`('%v','%v')`, v.Site, v.Sloc)
		siteSloc = append(siteSloc, key)
	}

	qryI := `select msrr.req_no_recv as req_no, msrr.create_dtm as create_dtm, msr.article_id, msr.batch, msr.serial, 
	msr.mfg_date, msr.exp_date, msr.recv_qty as stock_qty, msr.recv_unit as base_unit, msr.bin_location, msr.site as site, msr.sloc as sloc, msr.seq_no
	from m_stock_req_recv msrr 
	inner join m_stock_receive msr on msr.req_no_recv = msrr.req_no_recv `

	qryO := `select msrw.req_no_wd as req_no, msrw.create_dtm as create_dtm, msw.article_id, msw.batch, msw.serial, 
	msw.mfg_date, msw.exp_date, msw.wd_qty as stock_qty, msw.wd_unit as base_unit, msw.bin_location, msw.site as site, msw.sloc as sloc, msw.seq_no
	from m_stock_req_wd msrw 
	inner join m_stock_withdraw msw on msw.req_no_wd = msrw.req_no_wd `

	var qry, whereStr, dateStr string
	switch req.TypeReport {
	case "i":
		qry = qryI
		whereStr = `msrr.req_no_recv`
		dateStr = `msrr.create_dtm`
	case "o":
		qry = qryO
		whereStr = `msrw.req_no_wd`
		dateStr = `msrw.create_dtm`
	case "all":
		qry = fmt.Sprintf("select * from (%s union all %s) as combined_results", qryI, qryO)
		whereStr = `req_no`
		dateStr = `create_dtm`
	}

	qry += fmt.Sprintf(` where (site,sloc) in (%v)`, strings.Join(siteSloc, `,`))
	if len(reqNo) > 0 {
		qry += fmt.Sprintf(` and %v in ('%v')`, whereStr, strings.Join(reqNo, `','`))
	}

	switch {
	case req.BeginDate != nil && req.EndDate != nil:
		qry += fmt.Sprintf(` and (%v at time zone 'Asia/Bangkok')::date between '%v' and '%v'`, dateStr, req.BeginDate.Local().Format(timex.YYYYMMDD), req.EndDate.Local().Format(timex.YYYYMMDD))
	case req.BeginDate != nil && req.EndDate == nil:
		qry += fmt.Sprintf(` and date(%v) = '%v'`, dateStr, req.BeginDate.Local().Format(timex.YYYYMMDD))
	case req.BeginDate == nil && req.EndDate == nil && len(reqNo) == 0:
		qry += fmt.Sprintf(` order by %v desc limit 10`, dateStr)
	}

	return qry, nil
}

func processQueryResults(rows []sqlx.Map) []ReportList {
	var list []ReportList
	usedReq := make(map[string]bool)
	saleRepresent, _ := cuarticlemaster.SALES_REPRESENTATIVE.GetTableR()

	for _, row := range rows {
		reqNo := row.String(`req_no`)

		if !usedReq[reqNo] {
			var items []ReportDetail // Reset items for each reqNo
			usedReq[reqNo] = true

			for _, r := range rows {
				if r.String(`req_no`) == reqNo {
					articleID := r.String(`article_id`)
					baseUnit := r.String(`base_unit`)

					prod := cuarticlemaster.Products(articleID)
					articleName, mcCode := "", ""
					if prod != nil {
						articleName = prod.String(`name_th`)
						mcCode = prod.String(`merchandise_category2`)
					}

					mcName := ""
					mc := cuarticlemaster.ProductCategoriesMC(mcCode)
					if mc != nil {
						mcCodeShort := mc.String("mc_code")
						if len(mcCodeShort) > 3 {
							mcCodeShort = mcCodeShort[:3]
						}
						mcName = mcCodeShort + " " + mc.String("mc_name_th")
					}

					rtName := ""
					if saleRepresent != nil {
						saleRepx := saleRepresent.FindRow(func(m *sqlx.Map) bool {
							return m.String("id") == prod.String(`zmm_seller`)
						})
						if saleRepx != nil {
							rtName = saleRepx.String(`seller_code`) + " " + saleRepx.String(`seller_name`)
						}
					}

					mfgDate := ""
					if r.TimePtr(`mfg_date`) != nil {
						mfgDate = r.TimePtr(`mfg_date`).Local().Format(timex.YYYYMMDD)
					}

					expDate := ""
					if r.TimePtr(`exp_date`) != nil {
						expDate = r.TimePtr(`exp_date`).Local().Format(timex.YYYYMMDD)
					}

					items = append(items, ReportDetail{
						SeqNo:      r.Int(`seq_no`),
						Site:       r.String(`site`),
						Sloc:       r.String(`sloc`),
						RtName:     rtName,
						McName:     mcName,
						ArticleID:  r.String(`article_id`),
						ArtcleName: articleName,
						Batch:      r.String(`batch`),
						Serial:     r.String(`serial`),
						MfgDate:    mfgDate,
						ExpDate:    expDate,
						StockQty:   r.Float(`stock_qty`),
						BaseUnit:   cuarticlemaster.UnitByKey(baseUnit).String(`name_th`),
						Binloc:     r.String(`bin_location`),
					})
				}
			}

			list = append(list, ReportList{
				ReqNo:        reqNo,
				CreateDtm:    row.TimePtr(`create_dtm`).Format(timex.YYYYMMDD),
				ReportDetail: items,
			})
		}
	}

	return list
}
