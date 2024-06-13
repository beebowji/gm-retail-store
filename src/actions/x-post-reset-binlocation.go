package actions

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gitlab.dohome.technology/dohome-2020/gm-retail-store/src/common"
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gms"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/jwtx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
	cucompany "gitlab.dohome.technology/dohome-2020/go-structx/cu-company"
	gmauthen "gitlab.dohome.technology/dohome-2020/go-structx/gm-authen"
	"gitlab.dohome.technology/dohome-2020/go-structx/sappix"
)

type _X_POST_RESET_BINLOCATION_RQB struct {
	Site     string `json:"site"`
	Sloc     string `json:"sloc"`
	Password string `json:"password"`
}

func XPostResetBinlocation(c *gwx.Context) (any, error) {

	var dto _X_POST_RESET_BINLOCATION_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	// Validate
	if ex := c.Empty(dto.Site, `กรุณาระบุ Site`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(dto.Sloc, `กรุณาระบุ Sloc`); ex != nil {
		return nil, ex
	}

	// Get user
	userLogin, ex := jwtx.GetLoginInfo(c)
	if ex != nil {
		return nil, ex
	}
	userId := userLogin.UserInfo.UserID

	employee := cucompany.EmployeesX(userId)
	if userId != employee.String(`PersonID`) {
		return nil, fmt.Errorf("ไม่พบรหัสพนักงาน[%v], กรุณาตรวจสอบ", userId) // errorx.Unauthorized(`ไม่พบรหัสพนักงาน[%v], กรุณาตรวจสอบ`, userId)
	}

	// ลาออกแล้ว ?
	empStatus := employee.String(`EmployeeStatus`)
	if empStatus != "Active" {
		return nil, fmt.Errorf("รหัสพนักงาน[%v] ลาออกแล้ว(%v), กรุณาตรวจสอบ", userId, empStatus) //errorx.Unauthorized(`รหัสพนักงาน[%v] ลาออกแล้ว(%v), กรุณาตรวจสอบ`, userId, empStatus)
	}

	passwordEncrypt := employee.String(`Password`)
	if validx.IsEmpty(passwordEncrypt) {
		return nil, fmt.Errorf("รหัสพนักงาน[%v] ไม่พบรหัสผ่านในระบบ, กรุณาตรวจสอบ", userId) //errorx.Unauthorized(`รหัสพนักงาน[%v] ไม่พบรหัสผ่านในระบบ, กรุณาตรวจสอบ`, userId)
	}

	// Compare
	// if err := bcrypt.CompareHashAndPassword([]byte(passwordEncrypt), []byte(dto.Password)); err != nil {
	// 	return nil, fmt.Errorf(`รหัสพนักงาน[%v] รหัสผ่านไม่ถูกต้อง`, userId) //errorx.Unauthorized(`รหัสพนักงาน[%v] รหัสผ่านไม่ถูกต้อง`, userId)
	// }

	// .
	// 	PayloadJson(&gmauthen.LoginDto{
	// 		UserID:   dto.Username,
	// 		Password: dto.Password,
	// 		ModuleId: constants.DEC_QUICK_SHOP,
	// 	}

	ex = gms.GM_AUTHEN.HttpPost(`oauth2/login`).
		PayloadJson(&gmauthen.LoginDto{
			UserID:   userId,
			Password: dto.Password,
			ModuleId: `win-bof`,
		}).Do().Error()

	//ex = gms.GM_AUTHEN.HttpGet(`oauth2/login-token?moduleId=web-retail-store`).PayloadRequest(c.Request).Do().Error()

	if ex != nil {
		return nil, fmt.Errorf(`รหัสพนักงาน[%v] รหัสผ่านไม่ถูกต้อง`, userId)
	}

	// ตรวจสอบสิทธิ์ site sloc
	permission, ex := common.GetSiteSlocAuth(userLogin, dto.Site, dto.Sloc)
	if ex != nil {
		return nil, ex
	}
	if !permission {
		return nil, fmt.Errorf("you do not have permissions to this site sloc: %v|%v", dto.Site, dto.Sloc)
	}

	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}
	dxCompany, ex := sqlx.ConnectPostgresRW(dbs.DH_COMPANY)
	if ex != nil {
		return nil, ex
	}

	// Get table
	mResetHistoryTable, ex := dx.TableEmpty(`m_reset_history`)
	if ex != nil {
		return nil, ex
	}

	// query m_stock
	// queryMStock := fmt.Sprintf(`select * from m_stock where site = '%v' and sloc = '%v'`, dto.Site, dto.Sloc)
	// rowMStock, ex := dx.QueryScan(queryMStock)
	// if ex != nil {
	// 	return nil, ex
	// }

	// set sap เพื่อยิงไปยกลิกผูกตำแหน่ง
	saps := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB{}
	saps.IN_USERNAME = userId
	saps.IN_TCODE = "M_Stock"
	saps.IN_STORAGE_LOC = dto.Sloc
	saps.IN_WERKS = dto.Site
	saps.IV_MODE = "R"

	//var delKeys []string
	//for _, v := range rowMStock.Rows {
	//articleNo := v.String(`article_id`)
	//binLoc := v.String(`bin_location`)

	item := sappix.ZDD_HH_PROCESS_ASSIGNLOC_RQB_I_ARTICLE_ASSIGNLOC{
		IN_BIN_CODE:      "M",
		IN_MATNR:         "",
		IN_SITE:          dto.Site,
		IN_STORAGE_LOC:   dto.Sloc,
		IN_UNITOFMEASURE: "",
	}
	saps.I_ARTICLE_ASSIGNLOC.Item = append(saps.I_ARTICLE_ASSIGNLOC.Item, item)

	//key := fmt.Sprintf(`('%v','%v')`, articleNo, binLoc)
	//delKeys = append(delKeys, key)
	//}

	// to Saps
	if len(saps.I_ARTICLE_ASSIGNLOC.Item) > 0 {
		resp, ex := sappix.ZDD_HH_PROCESS_ASSIGNLOC(nil, saps)
		if ex != nil {
			return nil, ex
		}
		// Check SAP response for binding
		success, errMsg := common.IsSAPResponseSuccessful(resp)
		if !success {
			return nil, fmt.Errorf("SAP: เกิดข้อผิดพลาด (%v)", errMsg)
		}
	}

	// set data to table m_reset_history
	mResetHistoryTable.Rows = append(mResetHistoryTable.Rows, sqlx.Map{
		`id`:         uuid.New(),
		`user_id`:    userId,
		`site`:       dto.Site,
		`sloc`:       dto.Sloc,
		`create_dtm`: time.Now(),
	})

	if ex := dxCompany.Transaction(func(t *sqlx.Tx) error {
		// delete bin_location
		deleteQuery := fmt.Sprintf(`delete from bin_location where site = '%v' and sloc = '%v' and substr(bin_code,1,1) = 'M'`, dto.Site, dto.Sloc)
		_, ex = t.Exec(deleteQuery)
		if ex != nil {
			return ex
		}

		if ex = dx.Transaction(func(r *sqlx.Tx) error {
			// delete m_stock
			deleteQuery = fmt.Sprintf(`delete from m_stock where site = '%v' and sloc = '%v' and substr(bin_location,1,1) = 'M'`, dto.Site, dto.Sloc)
			_, ex = r.Exec(deleteQuery)
			if ex != nil {
				return ex
			}
			// insert log m_stock_transfer
			if len(mResetHistoryTable.Rows) > 0 {
				_, ex = r.InsertCreateBatches(`m_reset_history`, mResetHistoryTable, 100)
				if ex != nil {
					return ex
				}
			}

			return nil
		}); ex != nil {
			return ex
		}

		return nil
	}); ex != nil {
		return nil, ex
	}

	return nil, nil

}
