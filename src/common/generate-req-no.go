package common

import (
	"fmt"
	"strconv"
	"time"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
)

func GenerateReqNo(site, code string) string {

	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return ""
	}

	// ถ้าส่งมาครั้งแรกให้หาตัวเก่าก่อน
	var rowMStockReq *sqlx.Rows
	var key, keyForCode string

	if code == "O" {
		key = "req_no_wd"

		qryMStockReqWd := fmt.Sprintf(`select req_no_wd from m_stock_req_wd
			where SUBSTRING(req_no_wd from 2 for 4) = '%v'
			order by req_no_wd desc`, site)
		rowMStockReq, ex = dx.QueryScan(qryMStockReqWd)
		if ex != nil {
			return ""
		}
	} else if code == "I" {
		key = "req_no_recv"

		qryMStockReqWd := fmt.Sprintf(`select req_no_recv from m_stock_req_recv
			where SUBSTRING(req_no_recv from 2 for 4) = '%v'
			order by req_no_recv desc`, site)
		rowMStockReq, ex = dx.QueryScan(qryMStockReqWd)
		if ex != nil {
			return ""
		}
	}

	if len(rowMStockReq.Rows) > 0 {
		keyForCode = rowMStockReq.Rows[0].String(key)
	} else { // ถ้าเป็นการลงครั้งแรกจะคิวรี่ไม่เจอ
		keyForCode = code + site + "00000000"
	}

	// Extracting information from the selected previous code
	branchCode := keyForCode[1:5]
	lastDigits := keyForCode[9:13]

	// Remove leading zeros, increment, and add leading zeros again
	lastNum, _ := strconv.Atoi(lastDigits)
	lastNum++
	newLastDigits := fmt.Sprintf("%04d", lastNum)

	// Getting the current year and month
	currentYear := time.Now().Year() % 100
	currentMonth := time.Now().Month()

	// Formatting the components into the new code
	nextCode := fmt.Sprintf("%s%s%02d%02d%s", code, branchCode, currentYear, int(currentMonth), newLastDigits)

	return nextCode
}
