package actions

import (
	"fmt"
	"regexp"
	"strconv"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
	curetailstore "gitlab.dohome.technology/dohome-2020/go-structx/cu-retail-store"
	dhretailstore "gitlab.dohome.technology/dohome-2020/go-structx/dh-retail-store"
)

type ReasonItem struct {
	ReasonId   string `json:"reason_id"`
	ReasonName string `json:"reason_name"`
	VEntry     bool   `json:"v_entry"`
	VWithdraw  bool   `json:"v_withdraw"`
	VrTran     bool   `json:"vr_tran"`
	VrOften    bool   `json:"vr_often"`
	VrWh       bool   `json:"vr_wh"`
	VrSnm      bool   `json:"vr_snm"`
	UseFlag    string `json:"use_flag"`
}

type _X_SUBMIT_REASON_RQB struct {
	ReasonType string       `json:"reason_type"`
	ReasonList []ReasonItem `json:"reason_list"`
}

func XSubmitReason(c *gwx.Context) (any, error) {

	var dto _X_SUBMIT_REASON_RQB
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	// validate
	if ex := c.Empty(dto.ReasonType, `กรุณาระบุ ReasonType`); ex != nil {
		return nil, ex
	}

	if len(dto.ReasonList) == 0 {
		return nil, c.Error("กรุณาระบุข้อมูลใน ReasonList").StatusBadRequest()
	} else {
		for _, v := range dto.ReasonList {
			if ex := c.Empty(v.ReasonName, `กรุณาระบุ ReasonName`); ex != nil {
				return nil, ex
			}
			if ex := c.Empty(v.UseFlag, `กรุณาระบุ UseFlag`); ex != nil {
				return nil, ex
			}
		}
	}

	// Connect
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}

	// Get table
	mStockReasonTable, ex := dx.TableEmpty(dhretailstore.MStockReason{}.TableName())
	if ex != nil {
		return nil, ex
	}

	// qry chk reasonId
	qry := fmt.Sprintf(`select reason_id from m_stock_reason where reason_type = '%v' order by reason_id DESC`, dto.ReasonType)
	rowReason, ex := dx.QueryScan(qry)
	if ex != nil {
		return nil, ex
	}

	// Check if all ReasonIds are present
	missingReasons := make([]string, 0)
	for _, id := range filterNonEmpty(dto.ReasonList) {
		if !containsReasonId(rowReason, id) {
			missingReasons = append(missingReasons, id)
		}
	}

	// If there are missing ReasonIds, return an error
	if len(missingReasons) > 0 {

		return nil, c.Error(fmt.Sprintf("ReasonIds not found: %v", missingReasons)).StatusBadRequest()
	}

	// Extract numeric part from reason_id for gen new id
	numericPart := extractNumericPart(rowReason.Rows[0].String(`reason_id`))

	for _, v := range dto.ReasonList {
		reasonIdStg := v.ReasonId
		if validx.IsEmpty(reasonIdStg) {
			numericPart, _ = incrementNumericPart(numericPart)
			reasonIdStg = fmt.Sprintf("%s%s", dto.ReasonType, numericPart)
		}

		mStockReasonTable.Rows = append(mStockReasonTable.Rows, sqlx.Map{
			`reason_id`:   reasonIdStg,
			`reason_type`: dto.ReasonType,
			`reason_name`: v.ReasonName,
			`v_entry`:     v.VEntry,
			`v_withdraw`:  v.VWithdraw,
			`vr_tran`:     v.VrTran,
			`vr_often`:    v.VrOften,
			`vr_wh`:       v.VrWh,
			`vr_snm`:      v.VrSnm,
			`use_flag`:    v.UseFlag,
		})
	}

	// Update xd_sorting_confirm
	if len(mStockReasonTable.Rows) > 0 {
		_, ex = dx.InsertUpdateBatches(`m_stock_reason`, mStockReasonTable, []string{`reason_id`}, 100)
		if ex != nil {
			return nil, ex
		}
	}

	ex = ClearCache()
	if ex != nil {
		return nil, ex
	}

	return dto, nil
}

// Function to extract numeric part from a string
func extractNumericPart(s string) string {
	re := regexp.MustCompile(`(\d+)`)
	match := re.FindStringSubmatch(s)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// Function to increment the numeric part by 1
func incrementNumericPart(numericPart string) (string, error) {
	number, err := strconv.Atoi(numericPart)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%02d", number+1), nil
}

// Helper function to filter non-empty ReasonIds
func filterNonEmpty(reasonList []ReasonItem) []string {
	var result []string
	for _, v := range reasonList {
		if !validx.IsEmpty(v.ReasonId) {
			result = append(result, v.ReasonId)
		}
	}
	return result
}

// Helper function to check if a ReasonId is present in the rows
func containsReasonId(rows *sqlx.Rows, reasonId string) bool {
	for _, row := range rows.Rows {
		if row.String(`reason_id`) == reasonId {
			return true
		}
	}
	return false
}

func ClearCache() error {
	return curetailstore.M_STOCK_REASON.CacheDelRowAll()
}
