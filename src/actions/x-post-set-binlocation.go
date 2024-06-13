package actions

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
	"gitlab.dohome.technology/dohome-2020/go-structx/sappix"
)

type _X_POST_SET_BINLOCATION_RQB struct {
	Site            string `json:"site"`
	Sloc            string `json:"sloc"`
	Perid           string `json:"perid"`
	BinLocationList []struct {
		Binloc    string  `json:"binloc"`
		Loczone   string  `json:"loczone"`
		Lochshelf string  `json:"lochshelf"`
		Locside   string  `json:"locside"`
		Lochole   string  `json:"lochole"`
		Locclass  string  `json:"locclass"`
		Loctype   string  `json:"loctype"`
		Locwidth  float64 `json:"locwidth"`
		Lochigh   float64 `json:"lochigh"`
		Locdeep   float64 `json:"locdeep"`
	} `json:"bin_location_list"`
}

func XPostSetBinlocation(c *gwx.Context) (any, error) {

	var dto _X_POST_SET_BINLOCATION_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	// validate case ไหนก็ต้องส่ง
	if ex := c.Empty(dto.Site, `กรุณาระบุ Site`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(dto.Sloc, `กรุณาระบุ Sloc`); ex != nil {
		return nil, ex
	}

	// เช็ค data เพื่อดูว่าต้องเข้าเคสไหน
	if checkBinLocationList(dto) { // case เช็คว่ามีตำแหน่งนี้รึยัง แล้ว return เลย
		binLocList := getBinLocList(dto)

		// query
		exists, err := CheckBinLocationExistence(dto, binLocList)
		if err != nil {
			return nil, err
		}

		// loop chk
		for _, v := range exists.Rows {
			if validx.IsContains(binLocList, v.String(`binloc`)) { // ถ้ามีตำแหน่งนี้ให้ return error
				return nil, c.Error(fmt.Sprintf(`%v : ตำแหน่งนี้มีอยู่แล้ว`, v.String(`binloc`)))
			}
		}

		// ถ้าเช็คครบทุกตัวแล้วไม่มี error ให้จบ
		return nil, nil
	}

	// ถ้าไม่เข้า case check ตำแหน่ง ให้เช็คว่าต้องส่งมาครบทุกค่า
	if ex := c.Empty(dto.Perid, `กรุณาระบุ Perid`); ex != nil {
		return nil, ex
	}
	if len(dto.BinLocationList) == 0 {
		return nil, c.Error(`กรุณาระบุ BinLocationList`).StatusBadRequest()
	}
	if err := validateInput(dto); err != nil {
		return nil, err
	}

	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_COMPANY)
	if ex != nil {
		return nil, ex
	}
	dxRetailStore, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}

	// Get table
	binLocationMasterTable, ex := dx.TableEmpty(`bin_location_master`)
	if ex != nil {
		return nil, ex
	}
	createMLocHisTable, ex := dxRetailStore.TableEmpty(`create_m_location_history`)
	if ex != nil {
		return nil, ex
	}

	binLocList := getBinLocList(dto)

	// query
	exists, err := CheckBinLocationExistence(dto, binLocList)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	dateString := now.Format("2006-01-02")
	timeString := now.Format("15:04:05")

	saps := sappix.ZMMBAPI_CREATE_SLOC_RQB{}
	saps.IV_MODE = "I"

	for _, v := range dto.BinLocationList {
		// find หาว่ามีตำแหน่งนี้อยู่แล้วมั้ย
		data := exists.FindRow(func(m *sqlx.Map) bool {
			return m.String(`binloc`) == v.Binloc
		})
		if data != nil {
			return nil, c.Error(fmt.Sprintf("binloc %s มีอยู่แล้ว", v.Binloc)).StatusBadRequest()
		}

		// bin_location_master
		binLocationMasterTable.Rows = append(binLocationMasterTable.Rows, sqlx.Map{
			`loczone`:    v.Loczone,
			`lochshelf`:  v.Lochshelf,
			`locside`:    v.Locside,
			`lochole`:    v.Lochole,
			`locclass`:   v.Locclass,
			`loctype`:    v.Loctype,
			`werks`:      dto.Site,
			`lgort`:      dto.Sloc,
			`binloc`:     v.Binloc,
			`locwidth`:   v.Locwidth,
			`lochigh`:    v.Lochigh,
			`locdeep`:    v.Locdeep,
			`aprvflag`:   "",
			`useflag`:    "",
			`usercreate`: dto.Perid,
			`createdate`: dateString,
			`createtime`: timeString,
			`apprvdate`:  "",
			`apprvtime`:  "",
			`perid`:      dto.Perid,
			`command`:    "",
			`locdg_code`: "",
		})

		// set sap
		item := sappix.ZMMBAPI_CREATE_SLOC_RQB_T_LOCSTRC{
			LOCZONE:    v.Loczone,
			LOCHSHELF:  v.Lochshelf,
			LOCSIDE:    v.Locside,
			LOCHOLE:    v.Lochole,
			LOCCLASS:   v.Locclass,
			LOCTYPE:    v.Loctype,
			WERKS:      dto.Site,
			LGORT:      dto.Sloc,
			BINLOC:     v.Binloc,
			LOCWIDTH:   v.Locwidth,
			LOCHIGH:    v.Lochigh,
			LOCDEEP:    v.Locdeep,
			USERCREATE: dto.Perid,
			CREATEDATE: dateString,
			CREATETIME: timeString,
		}
		saps.T_LOCSTRC.Item = append(saps.T_LOCSTRC.Item, item)
	}

	// sap
	chkStatus := true
	var resp *sappix.ZMMBAPI_CREATE_SLOC_RSB
	var textError []string
	if len(saps.T_LOCSTRC.Item) > 0 {
		resp, ex = sappix.ZMMBAPI_CREATE_SLOC(nil, saps)
		if ex != nil {
			return nil, ex
		}

		// ลูปเช็คก่อนว่ามีตัวที่ error มั้ย
		for _, v := range resp.T_ERROR.Item {
			createMLocHisTable.Rows = append(createMLocHisTable.Rows, sqlx.Map{
				`id`:           uuid.New(),
				`bin_location`: v.LOCATION,
				`create_by`:    dto.Perid,
				`create_dtm`:   now,
				`status`:       "success",
			})

			if v.TYPE != "S" {
				chkStatus = false
				createMLocHisTable.Rows = nil
				break
			}
		}

		// ถ้ามีตัวที่ error ให้เก็บเฉพาะตัวที่ error
		if !chkStatus {
			for _, v := range resp.T_ERROR.Item {
				if v.TYPE != "S" {
					createMLocHisTable.Rows = append(createMLocHisTable.Rows, sqlx.Map{
						`id`:           uuid.New(),
						`bin_location`: v.LOCATION,
						`create_by`:    dto.Perid,
						`create_dtm`:   now,
						`status`:       "error",
					})

					textError = append(textError, fmt.Sprintf(`SAP %v: %v `, v.LOCATION, v.MESSAGE))
				}
			}
		}
	}

	// ถ้าส่ง sap ผ่านหมด ให้ insert bin_location_master
	if chkStatus {
		if len(binLocationMasterTable.Rows) > 0 {
			colConf := []string{`loczone`, `lochshelf`, `locside`, `lochole`, `locclass`, `loctype`, `werks`, `lgort`, `binloc`}
			_, ex = dx.InsertUpdateBatches(`bin_location_master`, binLocationMasterTable, colConf, 100)
			if ex != nil {
				return nil, ex
			}
		}
	}

	// insert history
	if len(createMLocHisTable.Rows) > 0 {
		_, ex = dxRetailStore.InsertCreateBatches(`create_m_location_history`, createMLocHisTable, 100)
		if ex != nil {
			return nil, ex
		}
	}

	if len(textError) > 0 {
		return nil, c.Error(fmt.Sprintf("%v", strings.Join(textError, `,`))).StatusBadRequest()
	}

	return nil, nil
}

func checkBinLocationList(input _X_POST_SET_BINLOCATION_RQB) bool {
	for _, item := range input.BinLocationList {
		if item.Binloc == "" {
			return false
		}
		// Check if other fields have null or empty values
		if item.Loczone != "" || item.Lochshelf != "" || item.Locside != "" ||
			item.Lochole != "" || item.Locclass != "" || item.Loctype != "" ||
			item.Locwidth != 0 || item.Lochigh != 0 || item.Locdeep != 0 {
			return false
		}
	}
	return true
}

func CheckBinLocationExistence(dto _X_POST_SET_BINLOCATION_RQB, binLoc []string) (*sqlx.Rows, error) {
	query := fmt.Sprintf(`SELECT binloc FROM bin_location_master WHERE aprvflag <> 'X' and werks = '%v' AND lgort = '%v' AND binloc IN ('%v')`, dto.Site, dto.Sloc, strings.Join(binLoc, `','`))
	row, ex := dbs.DH_COMPANY_R.QueryScan(query)
	if ex != nil {
		return nil, ex
	}
	return row, nil
}

// getBinLocList extracts bin locations from the input DTO
func getBinLocList(dto _X_POST_SET_BINLOCATION_RQB) []string {
	var binLocs []string
	for _, v := range dto.BinLocationList {
		binLocs = append(binLocs, v.Binloc)
	}
	return binLocs
}

func validateInput(dto _X_POST_SET_BINLOCATION_RQB) error {
	for _, v := range dto.BinLocationList {
		if validx.IsEmpty(v.Binloc) || validx.IsEmpty(v.Loczone) || validx.IsEmpty(v.Lochshelf) ||
			validx.IsEmpty(v.Locside) || validx.IsEmpty(v.Lochole) || validx.IsEmpty(v.Locclass) ||
			validx.IsEmpty(v.Loctype) || v.Locwidth < 0 || v.Lochigh < 0 || v.Locdeep < 0 {
			return fmt.Errorf("กรุณาระบุข้อมูลที่จำเป็นให้ครบถ้วน")
		}
	}
	return nil
}
