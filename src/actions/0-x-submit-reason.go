package actions

// import (
// 	"errors"
// 	"strconv"
// 	"strings"

// 	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
// 	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
// 	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
// 	"gitlab.dohome.technology/dohome-2020/go-servicex/stringx"
// 	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
// 	dhretailstore "gitlab.dohome.technology/dohome-2020/go-structx/dh-retail-store"
// )

// type _REASON_LIST struct {
// 	ReasonName string `json:"reason_name"`
// 	ReasonId   string `json:"reason_id"`
// 	VrEntry    bool   `json:"vr_entry"`
// 	VrWithdraw bool   `json:"vr_withdraw"`
// 	VrTran     bool   `json:"vr_tran"`
// 	VrOften    bool   `json:"vr_often"`
// 	VrWh       bool   `json:"vr_wh"`
// 	VrSnm      bool   `json:"vr_snm"`
// }

// type _X_SUBMIT_REASON_RQB struct {
// 	ReasonType string         `json:"reason_type"`
// 	ReasonList []_REASON_LIST `json:"reason_list"`
// }

// func XSubmitReason(c *gwx.Context) (any, error) {

// 	var dto _X_SUBMIT_REASON_RQB
// 	if ex := c.ShouldBindJSON(&dto); ex != nil {
// 		return nil, ex
// 	}

// 	//validate
// 	if ex := c.Empty(dto.ReasonType, `Invalid reason_type`); ex != nil {
// 		return nil, ex
// 	}

// 	reasonType := strings.ToUpper(dto.ReasonType)

// 	//find max reason_id ที่ reason_type=i,o
// 	mStockReasons, ex := dbs.DH_RETAIL_STORE_R.QueryScan(`select *
// 												from m_stock_reason
// 												where reason_type=$1
// 												order by reason_id desc`, reasonType)
// 	if ex != nil {
// 		return nil, ex
// 	}

// 	//find reason_id ตัวล่าสุด
// 	var maxReasonId float64
// 	rows := mStockReasons.GetRow(0)
// 	if rows != nil {
// 		spiltReasonId := strings.Split(rows.String(`reason_id`), reasonType)
// 		if len(spiltReasonId) >= 1 {
// 			maxReasonId, ex = strconv.ParseFloat(spiltReasonId[1], 64)
// 			if ex != nil {
// 				return nil, ex
// 			}
// 		}
// 	}

// 	tableName := dhretailstore.MStockReason{}.TableName()
// 	rowsMstockReason := mStockReasons.New()
// 	for k, v := range dto.ReasonList {
// 		mStockReanson := sqlx.Map{
// 			`reason_name`: v.ReasonName,
// 			`reason_id`:   v.ReasonId,
// 			`reason_type`: reasonType,
// 			`vr_entry`:    v.VrEntry,
// 			`vr_withdraw`: v.VrWithdraw,
// 			`vr_tran`:     v.VrTran,
// 			`vr_often`:    v.VrOften,
// 			`vr_wh`:       v.VrWh,
// 			`vr_snm`:      v.VrSnm,
// 		}

// 		//create : reason_id=reason_id ตัวล่าสุด+1 //update เช็คว่ามี reason_id ใน db มั้ย
// 		if validx.IsEmpty(v.ReasonId) {
// 			maxReasonId += 1
// 			reasonId := reasonType + stringx.PadLeft(strconv.Itoa(int(maxReasonId)), 2, '0')
// 			mStockReanson.Set(`reason_id`, reasonId)
// 			dto.ReasonList[k].ReasonId = reasonId
// 		} else {
// 			if rowReason := mStockReasons.FindRow(func(m *sqlx.Map) bool {
// 				return m.String(`reason_id`) == v.ReasonId
// 			}); rowReason == nil {
// 				return nil, errors.New(`not found reason_id`)
// 			}
// 		}

// 		rowsMstockReason.Rows = append(rowsMstockReason.Rows, mStockReanson)
// 	}

// 	//insert update
// 	if len(rowsMstockReason.Rows) > 0 {
// 		if _, ex := dbs.DH_RETAIL_STORE_W.InsertUpdateBatches(tableName, rowsMstockReason, []string{`reason_id`}, 100); ex != nil {
// 			return nil, ex
// 		}
// 	}

// 	return &dto, nil
// }
