package actions

// import (
// 	"fmt"
// 	"strings"

// 	"github.com/gin-gonic/gin"
// 	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
// 	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
// 	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
// )

// func XListReason(c *gwx.Context) (any, error) {

// 	reasonType := c.Query(`reason_type`)
// 	page := c.Query(`page`)

// 	query := `select * from m_stock_reason`
// 	if !validx.IsEmpty(reasonType) {
// 		query += fmt.Sprintf(` where reason_type='%v'`, strings.ToUpper(reasonType))
// 	}
// 	if !validx.IsEmpty(page) {
// 		if validx.IsEmpty(reasonType) {
// 			query += ` where`
// 		} else {
// 			query += ` and`
// 		}
// 		query += ` %v=true`
// 	}

// 	rowsMStockReason, ex := dbs.DH_RETAIL_STORE_R.QueryScan(query)
// 	if ex != nil {
// 		return nil, ex
// 	}

// 	if len(rowsMStockReason.Rows) == 0 {
// 		return nil, c.ErrorBadRequest(`ไม่พบข้อมูล`)
// 	}

// 	return gin.H{"reason_list": rowsMStockReason.Rows}, nil
// }
