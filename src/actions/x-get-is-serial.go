package actions

import (
	"fmt"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
)

type _X_GET_IS_SERIAL_RSB struct {
	IsSerialCorrect bool `json:"is_serial_correct"`
}

func XGetIsSerial(c *gwx.Context) (any, error) {

	serialNo := c.Query("serial_no")
	articleId := c.Query("article_id")
	siteCode := c.Query("site_code")
	slocCode := c.Query("sloc_code")
	actionCode := c.Query("action")

	// Validate query parameters
	parameters := map[string]string{
		"serial_no":  serialNo,
		"article_id": articleId,
		"site_code":  siteCode,
		"sloc_code":  slocCode,
	}
	for param, value := range parameters {
		if ex := c.Empty(value, fmt.Sprintf("กรุณาระบุ %s", param)); ex != nil {
			return nil, ex
		}
	}

	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_COMMERCE)
	if ex != nil {
		return nil, ex
	}

	dm, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}

	// Prepare the query
	qry := `SELECT EXISTS (
		SELECT 1 FROM stock_serials 
		WHERE serial_no = $1 AND article_id = $2 AND site_code = $3 AND sloc_code = $4
	)`

	// Execute the query
	var isSerial bool
	if err := dx.QueryRow(qry, serialNo, articleId, siteCode, slocCode).Scan(&isSerial); err != nil {
		return nil, err
	}

	// Prepare the response
	response := _X_GET_IS_SERIAL_RSB{
		IsSerialCorrect: isSerial,
	}

	qryMStock := ` 
		SELECT 1 FROM m_stock 
		WHERE serial = $1 
	`
	if actionCode == "W" {
		qryMStock += fmt.Sprintf(" AND article_id = '%v' AND site = '%v' AND sloc = '%v' ", articleId, siteCode, slocCode)
	}

	qryMStock = "SELECT EXISTS ( " + qryMStock + " )"

	// Execute the query
	var isSerialInStock bool
	if err := dm.QueryRow(qryMStock, serialNo).Scan(&isSerialInStock); err != nil {
		return nil, err
	}

	if actionCode == "I" && isSerialInStock {
		response.IsSerialCorrect = false
		return response, fmt.Errorf("serial %s ถูกจัดเก็บไปแล้ว", serialNo)
	}
	if actionCode == "W" && !isSerialInStock {
		response.IsSerialCorrect = false
		return response, fmt.Errorf("serial %s ไม่ในสต็อค", serialNo)

	}

	return response, nil
}
