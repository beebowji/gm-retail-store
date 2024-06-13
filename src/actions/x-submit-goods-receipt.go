package actions

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/errorx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gms"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/stringx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/timex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tox"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
	cuarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/cu-article-master"
	dhretailstore "gitlab.dohome.technology/dohome-2020/go-structx/dh-retail-store"
	gmarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/gm-article-master"
)

type _SUBMIT_GOODS_RECEIPT struct {
	ArticleId    string   `json:"article_id"`
	BatchNo      string   `json:"batch_no"`
	SerialNo     string   `json:"serial_no"`
	MfgDate      string   `json:"mfg_date"`
	ExpDate      string   `json:"exp_date"`
	BinLocation  string   `json:"bin_location"`
	RecvQty      *float64 `json:"recv_qty"`
	RecvUnit     string   `json:"recv_unit"`
	RecvReasonId string   `json:"recv_reason_id"`
	Remark       string   `json:"remark"`
}

type _X_SUBMIT_GOODS_RECEIPT_RQB struct {
	Site        string                  `json:"site"`
	Sloc        string                  `json:"sloc"`
	CreateBy    string                  `json:"create_by"`
	ReceiveList []_SUBMIT_GOODS_RECEIPT `json:"receive_list"`
}

func XSubmitGoodsReceipt(c *gwx.Context) (any, error) {

	var dto _X_SUBMIT_GOODS_RECEIPT_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	//validate
	if ex := validate(&dto); ex != nil {
		return nil, ex
	}

	//cu sale-rep
	salesRepresentative, ex := cuarticlemaster.SalesRepresentative()
	if ex != nil {
		return nil, ex
	}
	mapSaleRep := map[string]sqlx.Map{}
	for _, v := range salesRepresentative.Rows {
		mapSaleRep[v.String(`id`)] = v
	}

	//find mstock
	articleIds := []string{}
	for _, v := range dto.ReceiveList {
		articleId := v.ArticleId
		if !validx.IsContains(articleIds, articleId) {
			articleIds = append(articleIds, articleId)
		}
	}
	query := fmt.Sprintf(`select * from m_stock where article_id in ('%v')`, strings.Join(articleIds, `','`))
	mstock, ex := dbs.DH_RETAIL_STORE_R.QueryScan(query)
	if ex != nil {
		return nil, ex
	}

	mapMstock := map[string]sqlx.Map{}
	for _, v := range mstock.Rows {
		key := fmt.Sprintf(`%v|%v|%v|%v|%v|%v|%v`, v.String(`site`), v.String(`sloc`), v.String(`article_id`), v.String(`batch`),
			v.String(`serial`), v.Time(`exp_date`).Format(timex.YYYYMMDD), v.String(`bin_location`))
		mapMstock[key] = v
	}

	site := dto.Site
	sloc := dto.Sloc
	createBy := dto.CreateBy
	timeNow := time.Now().UTC()
	rowsMStock, _ := dbs.DH_RETAIL_STORE_W.TableEmpty(dhretailstore.MStock{}.TableName())
	rowsMStockEntryHistory, _ := dbs.DH_RETAIL_STORE_W.TableEmpty(dhretailstore.MStockEntryHistory{}.TableName())

	//map m_stock -- m_stock_entry_history
	for _, v := range dto.ReceiveList {
		articleId := v.ArticleId
		batch := v.BatchNo
		serial := v.SerialNo
		binLocation := v.BinLocation

		newRow := sqlx.Map{
			`id`:             uuid.New(),
			`site`:           site,
			`sloc`:           sloc,
			`create_by`:      createBy,
			`article_id`:     articleId,
			`batch`:          batch,
			`serial`:         serial,
			`bin_location`:   binLocation,
			`recv_qty`:       v.RecvQty,
			`recv_unit`:      v.RecvUnit,
			`recv_reason_id`: v.RecvReasonId,
			`remark`:         v.Remark,
			`create_dtm`:     timeNow,
			`update_dtm`:     timeNow,
			`update_by`:      createBy,
		}

		//
		mfgDate := tox.TimeSAP(v.MfgDate)
		if mfgDate != nil {
			newRow.Set(`mfg_date`, mfgDate.UTC())
		}

		//
		expDate := tox.TimeSAP(v.ExpDate)
		if expDate != nil {
			newRow.Set(`exp_date`, expDate.UTC())
		}

		//seller_cd,mc_cd,exp_date ถ้าเป็นว่าง,base_unit
		if ex := getDetailProduct(&newRow, mapSaleRep); ex != nil {
			return nil, ex
		}

		//เก็บ log
		rowsMStockEntryHistory.Rows = append(rowsMStockEntryHistory.Rows, newRow)

		//convert unit
		dtoConvert := gmarticlemaster.X_CONVERT_UNITS_RQB{}
		dtoConvert.Item = append(dtoConvert.Item, gmarticlemaster.X_CONVERT_UNITS_RQB_ITEM{
			ArticleId:  articleId,
			UnitCodeFr: v.RecvUnit,
			UnitCodeTo: newRow.String(`base_unit`),
			UnitAmtFr:  *v.RecvQty,
		})
		rtoConvert := gmarticlemaster.X_CONVERT_UNITS_RSB{}
		ex := gms.GM_ARTICLE_MASTER.HttpPost(`product-units/convert-units`).PayloadJson(dtoConvert).Do().Struct(&rtoConvert)
		if ex != nil {
			return nil, ex
		}
		if len(rtoConvert.Item) > 0 {
			convert := rtoConvert.Item[0]
			newRow.Set(`stock_qty`, convert.UnitAmtTo)
			newRow.Set(`base_unit`, convert.UnitCodeTo)
		}

		//ยุบรวมกัน ถ้า key 7 key ตรงกัน
		isFind := false
		expireDate := newRow.Time(`exp_date`).Format(timex.YYYYMMDD)
		keyMstock := fmt.Sprintf(`%v|%v|%v|%v|%v|%v|%v`, site, sloc, articleId, batch, serial, expireDate, binLocation)
		for _, v := range rowsMStock.Rows {
			if site == v.String(`site`) && sloc == v.String(`sloc`) && articleId == v.String(`article_id`) && batch == v.String(`batch`) && serial == v.String(`serial`) &&
				expireDate == v.Time(`exp_date`).Format(timex.YYYYMMDD) && binLocation == v.String(`bin_location`) {
				if _, ok := mapMstock[keyMstock]; ok {
					v.Set(`stock_qty`, v.Float(`stock_qty`)+newRow.Float(`stock_qty`))
				}
				isFind = true
			}
		}

		if !isFind {
			if mstock, ok := mapMstock[keyMstock]; ok {
				newRow.Set(`stock_qty`, mstock.Float(`stock_qty`)+newRow.Float(`stock_qty`))
			}
			rowsMStock.Rows = append(rowsMStock.Rows, newRow)
		}
	}

	//insert-update
	if len(rowsMStockEntryHistory.Rows) > 0 {
		if _, ex := dbs.DH_RETAIL_STORE_W.InsertUpdateBatches(dhretailstore.MStockEntryHistory{}.TableName(), rowsMStockEntryHistory, []string{`id`}, 100); ex != nil {
			return nil, ex
		}
	}

	if len(rowsMStock.Rows) > 0 {
		if _, ex := dbs.DH_RETAIL_STORE_W.InsertUpdateBatches(dhretailstore.MStock{}.TableName(), rowsMStock, []string{`site`, `sloc`, `article_id`, `batch`, `serial`, `exp_date`, `bin_location`}, 100); ex != nil {
			return nil, ex
		}
	}

	return nil, nil
}

func validate(dto *_X_SUBMIT_GOODS_RECEIPT_RQB) error {
	if validx.IsEmpty(dto.Site) {
		return errorx.New(`Invalid Site`).StatusBadRequest()
	}
	if validx.IsEmpty(dto.Sloc) {
		return errorx.New(`Invalid Sloc`).StatusBadRequest()
	}
	if validx.IsEmpty(dto.CreateBy) {
		return errorx.New(`Invalid CreateBy`).StatusBadRequest()
	}
	if len(dto.ReceiveList) == 0 {
		return errorx.New(`Invalid ReceiveList`).StatusBadRequest()
	}

	for _, v := range dto.ReceiveList {
		if validx.IsEmpty(v.ArticleId) {
			return errorx.New(`Invalid ArticleId`).StatusBadRequest()
		}
		if validx.IsEmpty(v.BinLocation) {
			return errorx.New(`Invalid BinLocation`).StatusBadRequest()
		}
		if v.RecvQty == nil {
			return errorx.New(`Invalid RecvQty`).StatusBadRequest()
		}
		if validx.IsEmpty(v.RecvUnit) {
			return errorx.New(`Invalid RecvUnit`).StatusBadRequest()
		}
		if validx.IsEmpty(v.MfgDate) && validx.IsEmpty(v.ExpDate) {
			return errorx.New(`Invalid MfgDate`).StatusBadRequest()
		}
	}

	return nil
}

func getDetailProduct(dto *sqlx.Map, cuSaleRep map[string]sqlx.Map) error {

	articleId := dto.String(`article_id`)
	if validx.IsEmpty(articleId) {
		return nil
	}

	rowsProduct, ex := dbs.DH_ARTICLE_MASTER_R.QueryScan(`select
														p.article_id,
														p.name_th,
														p.article_id,
														p.merchandise_category,
														p.zmm_seller,
														p.tot_shelf_life,
														u.unit_code,
														pu.unit_name
														from dh_article_master.public.products p 
														left join dh_article_master.public.product_units pu 
														on p.id=pu.products_id and pu.is_unit_base=true 
														left join dh_article_master.public.units u on pu.units_id=u.id 
														where article_id=$1`, articleId)
	if ex != nil {
		return ex
	}

	product := rowsProduct.GetRow(0)
	if product != nil {
		//base_unit
		dto.Set(`base_unit`, product.String(`unit_code`))
		//seller_cd
		sellerCd := stringx.SubString(product.String(`merchandise_category`), 0, 3)
		dto.Set(`seller_cd`, sellerCd)
		//mc_cd
		if saleRep, ok := cuSaleRep[product.String(`id`)]; ok {
			dto.Set(`mc_cd`, saleRep.String(`seller_code`))
		}
		//exp_date
		if dto.TimePtr(`exp_date`) == nil && dto.TimePtr(`mfg_date`) != nil {
			totShelfLife := product.Int(`tot_shelf_life`)
			expDate := dto.Time(`mfg_date`).AddDate(0, 0, totShelfLife)
			dto.Set(`exp_date`, expDate)
		}
	}

	return nil
}
