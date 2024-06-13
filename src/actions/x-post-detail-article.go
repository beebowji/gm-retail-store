package actions

import (
	"fmt"
	"sort"
	"time"

	"gitlab.dohome.technology/dohome-2020/gm-retail-store/src/common"
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/errorx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/timex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
)

type _X_POST_DETAIL_ARTICLE_RQB struct {
	ArticleId   string     `json:"article_id"`
	Site        string     `json:"site"`
	Sloc        string     `json:"sloc"`
	Batch       string     `json:"batch"`
	Serial      string     `json:"serial"`
	MfgDate     *time.Time `json:"mfg_date"`
	ExpDate     *time.Time `json:"exp_date"`
	BinLocation string     `json:"bin_location"`
}

type _X_POST_DETAIL_ARTICLE_RSB struct {
	ArticleID              string      `json:"article_id"`
	NameTh                 string      `json:"name_th"`
	BaseUnitCode           string      `json:"base_unit_code"`
	BaseUnitName           string      `json:"base_unit_name"`
	IsUnitBase             bool        `json:"is_unit_base"`
	IsRequireBatch         bool        `json:"is_require_batch"`
	IsRequireSerial        bool        `json:"is_require_serial"`
	IsRequireShelfLift     bool        `json:"is_require_shelf_lift"`
	BinLocation            string      `json:"bin_location"`
	StockQty               float64     `json:"stock_qty"`
	Batch                  string      `json:"batch"`
	Serial                 string      `json:"serial"`
	MfgDate                *time.Time  `json:"mfg_date"`
	ExpDate                *time.Time  `json:"exp_date"`
	UnitList               []UnitList  `json:"unit_list"`
	ListBinlocationSuggest []string    `json:"list_binlocation_suggest"`
	ShelfLife              []ShelfLife `json:"shelf_life"`
}

type UnitList struct {
	UnitCode string `json:"unit_code"`
	UnitName string `json:"unit_name"`
}

type ShelfLife struct {
	ExprireDate *time.Time `json:"exprire_date"`
	Day         int        `json:"day"`
}

// Define constants for serial codes.
const (
	SerialCodeZSD1 = "ZSD1"
	SerialCodeZSR1 = "ZSR1"
	SerialCodeZSR2 = "ZSR2"
)

func XPostDetailArticle(c *gwx.Context) (any, error) {
	var dto _X_POST_DETAIL_ARTICLE_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	if err := validateRequestDTArticle(dto, c); err != nil {
		return nil, err
	}

	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_COMPANY)
	if ex != nil {
		return nil, ex
	}
	dxArticleMaster, ex := sqlx.ConnectPostgresRW(dbs.DH_ARTICLE_MASTER)
	if ex != nil {
		return nil, ex
	}
	dxRetailStore, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}

	if err := validateBinLocation(dto, dx); err != nil {
		return nil, err
	}

	// Query product information
	product, err := queryProduct(dxArticleMaster, dto.ArticleId)
	if err != nil {
		return nil, err
	}

	// เช็คว่าเป็นสินค้ามีอายุรึเปล่า
	var shelfLife []ShelfLife
	if product.IsRequireShelfLift {
		// query cal ShelfLife
		querySf := fmt.Sprintf(`SELECT exp_date FROM m_stock 
		WHERE article_id = '%v' AND site = '%v' AND sloc = '%v' AND bin_location = '%v'`, dto.ArticleId, dto.Site, dto.Sloc, dto.BinLocation)
		rowSf, ex := dxRetailStore.QueryScan(querySf)
		if ex != nil {
			return _X_POST_DETAIL_ARTICLE_RSB{}, ex
		}

		if len(rowSf.Rows) > 0 {
			for _, v := range rowSf.Rows {
				if v.TimePtr(`exp_date`) != nil {
					daysLeft := common.CalculateDaysDiff(v.TimePtr(`exp_date`))

					shelfLife = append(shelfLife, ShelfLife{
						ExprireDate: v.TimePtr(`exp_date`),
						Day:         daysLeft,
					})
				}
			}

			// Sort the shelfLife slice by the Day field with the smallest number first
			sort.Slice(shelfLife, func(i, j int) bool {
				return shelfLife[i].Day < shelfLife[j].Day
			})
		}

		if dto.MfgDate != nil && dto.ExpDate == nil {
			dto.ExpDate = common.FindExpDate(dto.ArticleId, dto.MfgDate)
		}
	}

	// Query bin location suggest
	_, binLocation, err := queryBinLocationSuggest(dxRetailStore, dto.Site, dto.Sloc, dto.ArticleId, dto.BinLocation)
	if err != nil {
		return nil, err
	}

	stockQty, err := queryStockQuantity(dxRetailStore, dto, dto.BinLocation, product)
	if err != nil {
		return nil, err
	}

	response := createResponse(dto, product, stockQty, binLocation, shelfLife)
	return response, nil
}

func validateRequestDTArticle(dto _X_POST_DETAIL_ARTICLE_RQB, c *gwx.Context) error {
	if err := c.Empty(dto.ArticleId, `กรุณาระบุ ArticleID`); err != nil {
		return err
	}
	if err := c.Empty(dto.Site, `กรุณาระบุ Site`); err != nil {
		return err
	}
	if err := c.Empty(dto.Sloc, `กรุณาระบุ Sloc`); err != nil {
		return err
	}
	return nil
}

func validateBinLocation(dto _X_POST_DETAIL_ARTICLE_RQB, dx *sqlx.DB) error {
	if !validx.IsEmpty(dto.BinLocation) {
		query := fmt.Sprintf(`
			SELECT binloc, useflag
			FROM bin_location_master
			WHERE useflag = 'X' AND aprvflag = 'C' AND binloc = '%v' AND werks = '%v' AND lgort = '%v'`,
			dto.BinLocation, dto.Site, dto.Sloc)

		rows, err := dx.QueryScan(query)
		if err != nil {
			return err
		}

		if len(rows.Rows) == 0 {
			return fmt.Errorf("ไม่มีตำแหน่งจัดเก็บนี้")
		}
	}
	return nil
}

// Query product information
func queryProduct(dxArticleMaster *sqlx.DB, articleID string) (_X_POST_DETAIL_ARTICLE_RSB, error) {

	// query product
	qryProduct := fmt.Sprintf(`select p.article_id,
	p.name_th,
	u.unit_code,
	u.name_th as unit_name,
	ps.active_batch as is_require_batch,
	tc.item_code as serial_code,
	pu.is_unit_base,
	p.rem_shelf_life,
	p.tot_shelf_life
	from products p
	left join product_site ps on ps.products_id = p.id
	left join product_units pu on p.id = pu.products_id
	left join units u on pu.units_id = u.id
	left join topic_code tc on tc.id = ps.serial_type
	where p.article_id = '%v'`, articleID)
	rowProduct, ex := dxArticleMaster.QueryScan(qryProduct)
	if ex != nil {
		return _X_POST_DETAIL_ARTICLE_RSB{}, ex
	}
	if len(rowProduct.Rows) == 0 {
		return _X_POST_DETAIL_ARTICLE_RSB{}, errorx.New(`ไม่มีสินค้าในระบบ`).StatusBadRequest()
	}

	// Process product information
	return processProduct(rowProduct)
}

// Process product information
func processProduct(rowProduct *sqlx.Rows) (_X_POST_DETAIL_ARTICLE_RSB, error) {
	// Initialize variables
	var (
		isUnitBase, isRequireSerial, isRequireShelfLift, isRequireBatch bool
		unitCode, unitName                                              string
		unitList                                                        []UnitList
		unitMap                                                         = make(map[string]bool)
	)

	// Process each row
	for _, v := range rowProduct.Rows {
		// chk serial
		if v.String(`serial_code`) == SerialCodeZSD1 || v.String(`serial_code`) == SerialCodeZSR1 || v.String(`serial_code`) == SerialCodeZSR2 {
			isRequireSerial = true
		}

		// chk product type has an expiration date
		if v.Int(`rem_shelf_life`) > 0 && v.Int(`tot_shelf_life`) > 0 {
			isRequireShelfLift = true
		}

		// chk batch
		if v.Bool(`is_require_batch`) {
			isRequireBatch = true
		}

		// Check unit base
		if v.Bool(`is_unit_base`) && !isUnitBase {
			isUnitBase = true
			unitCode = v.String(`unit_code`)
			unitName = v.String(`unit_name`)
		}

		// Build unit list
		if !unitMap[v.String(`unit_code`)] {
			unitMap[v.String(`unit_code`)] = true
			unitList = append(unitList, UnitList{
				UnitCode: v.String(`unit_code`),
				UnitName: v.String(`unit_name`),
			})
		}
	}

	return _X_POST_DETAIL_ARTICLE_RSB{
		NameTh:             rowProduct.Rows[0].String(`name_th`),
		BaseUnitCode:       unitCode,
		BaseUnitName:       unitName,
		IsUnitBase:         isUnitBase,
		IsRequireBatch:     isRequireBatch,
		IsRequireSerial:    isRequireSerial,
		IsRequireShelfLift: isRequireShelfLift,
		UnitList:           unitList,
	}, nil
}

// Query bin location
func queryBinLocationSuggest(dx *sqlx.DB, site, sloc, articleID, binLocation string) (string, []string, error) {
	// query bin_location
	qryBinLoc := fmt.Sprintf(
		`select bin_location, 'X' as useflag
		from m_stock 
		where LEFT(bin_location, 1) = 'M' and site = '%v' and sloc = '%v' and article_id = '%v'`, site, sloc, articleID)
	rowBinLoc, ex := dx.QueryScan(qryBinLoc)
	if ex != nil {
		return "", nil, ex
	}

	// Process bin location
	return processBinLocation(rowBinLoc, binLocation)
}

// Process bin location
func processBinLocation(rowBinLoc *sqlx.Rows, binLoc string) (string, []string, error) {
	// Initialize variables
	var binLocation []string

	// Process each row
	binStr := binLoc
	for _, v := range rowBinLoc.Rows {
		// เช็คว่าตำแหน่งจัดเก็บนี้เปิดใช้งานหรือไม่ เช็คเฉพาะเคสที่ส่ง BinLoc เข้ามา
		if (validx.IsEmpty(v.String(`useflag`)) || v.String(`useflag`) != "X") && !validx.IsEmpty(binLoc) {
			return "", nil, fmt.Errorf("ตำแหน่งจัดเก็บนี้ไม่ได้มีการเปิดใช้งาน")
		}

		binCode := v.String(`bin_location`)

		if validx.IsEmpty(binStr) {
			binStr = binCode
		}

		if !validx.IsContains(binLocation, binCode) {
			binLocation = append(binLocation, binCode)
		}
	}

	return binStr, binLocation, nil
}

func queryStockQuantity(dx *sqlx.DB, dto _X_POST_DETAIL_ARTICLE_RQB, binStr string, product _X_POST_DETAIL_ARTICLE_RSB) (float64, error) {
	if (product.IsRequireBatch && validx.IsEmpty(dto.Batch)) ||
		(product.IsRequireSerial && validx.IsEmpty(dto.Serial)) ||
		(product.IsRequireShelfLift && dto.ExpDate == nil) {
		return 0, nil
	}

	query := fmt.Sprintf(`
		SELECT stock_qty, mfg_date, exp_date 
		FROM m_stock 
		WHERE article_id = '%v' AND site = '%v' AND sloc = '%v' AND bin_location = '%v'`,
		dto.ArticleId, dto.Site, dto.Sloc, binStr)

	switch {
	case product.IsRequireBatch:
		query += fmt.Sprintf(` AND batch = '%v'`, dto.Batch)
	case product.IsRequireSerial:
		query += fmt.Sprintf(` AND serial = '%v'`, dto.Serial)
	case product.IsRequireShelfLift:
		query += fmt.Sprintf(` AND exp_date::date = '%v'`, dto.ExpDate.Format(timex.YYYYMMDD))
	}

	rows, err := dx.QueryScan(query)
	if err != nil {
		return 0, err
	}

	if len(rows.Rows) == 0 {
		return 0, nil
	}

	dto.MfgDate = rows.Rows[0].TimePtr(`mfg_date`)
	dto.ExpDate = rows.Rows[0].TimePtr(`exp_date`)
	return rows.Rows[0].Float(`stock_qty`), nil
}

func createResponse(dto _X_POST_DETAIL_ARTICLE_RQB, product _X_POST_DETAIL_ARTICLE_RSB, stockQty float64, binLocation []string, shelfLife []ShelfLife) _X_POST_DETAIL_ARTICLE_RSB {
	//binStr = dto.BinLocation
	return _X_POST_DETAIL_ARTICLE_RSB{
		ArticleID:              dto.ArticleId,
		NameTh:                 product.NameTh,
		BaseUnitCode:           product.BaseUnitCode,
		BaseUnitName:           product.BaseUnitName,
		IsUnitBase:             product.IsUnitBase,
		IsRequireBatch:         product.IsRequireBatch,
		IsRequireSerial:        product.IsRequireSerial,
		IsRequireShelfLift:     product.IsRequireShelfLift,
		BinLocation:            dto.BinLocation,
		StockQty:               stockQty,
		Batch:                  dto.Batch,
		Serial:                 dto.Serial,
		MfgDate:                dto.MfgDate,
		ExpDate:                dto.ExpDate,
		UnitList:               product.UnitList,
		ListBinlocationSuggest: binLocation,
		ShelfLife:              shelfLife,
	}
}
