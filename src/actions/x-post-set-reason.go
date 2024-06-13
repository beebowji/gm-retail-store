package actions

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/errorx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/stringx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tox"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
)

type X_POST_SET_REASON_RQB Reason

type Reason struct {
	ReasonType string       `json:"reason_type"`
	ReasonList []ReasonList `json:"reason_list"`
	ReasonID   []string     `json:"-"`
}

type ReasonList struct {
	ReasonID   string `json:"reason_id"`
	ReasonName string `json:"reason_name"`
	UseFlag    string `json:"use_flag"`
	VrTran     *bool  `json:"vr_tran"`
	VrOften    *bool  `json:"vr_often"`
	VrWh       *bool  `json:"vr_wh"`
	VrSnm      *bool  `json:"vr_snm"`
}

func XPostSetReason(c *gwx.Context) (any, error) {

	// Bind
	var dto X_POST_SET_REASON_RQB
	if err := c.ShouldBindJSON(&dto); err != nil {
		return nil, err
	}

	// Validator
	if err := dto.validation(); err != nil {
		return nil, err
	}

	// Find
	query := `select * from m_stock_reason`
	if len(dto.ReasonID) > 0 {
		query += fmt.Sprintf(` where reason_id in ('%v')`, strings.Join(dto.ReasonID, `','`))
	}
	rReason, err := dbs.DH_RETAIL_STORE_R.QueryScan(query)
	if err != nil {
		return nil, err
	}
	rReason.BuildMap(func(m *sqlx.Map) string {
		return fmt.Sprintf(`%s|%s`, strings.ToUpper(m.String(`reason_id`)), strings.ToUpper(m.String(`reason_type`)))
	})

	// Get Maximum Running Type
	maxReason, err := getMaximumReason(dto.ReasonType)
	if err != nil {
		return nil, err
	}

	// Handle
	for i, r := range dto.ReasonList {
		// ถ้าส่ง reason_id มา
		if !validx.IsEmpty(r.ReasonID) {
			k := fmt.Sprintf(`%s|%s`, strings.ToUpper(r.ReasonID), strings.ToUpper(dto.ReasonType))
			found := rReason.FindMap(k)

			if found == nil {
				return nil, c.Error(fmt.Sprintf(`ไม่พบข้อมูล reason_id(%v)`, r.ReasonID)).StatusBadRequest()
			} else {
				updates := map[string]interface{}{
					"reason_name": r.ReasonName,
					"use_flag":    r.UseFlag,
					"vr_tran":     r.VrTran,
					"vr_often":    r.VrOften,
					"vr_snm":      r.VrSnm,
					"vr_wh":       r.VrWh,
				}

				for key, value := range updates {
					// Skip setting the value if it's a nil boolean
					if ptr, ok := value.(*bool); ok && ptr == nil {
						continue
					}

					found.Set(key, value)
				}
			}
		} else {
			maxReason++
			reasonID := fmt.Sprintf(`%s%v`, dto.ReasonType, stringx.PadLeft(tox.String(maxReason), 2, '0'))
			rReason.Rows = append(rReason.Rows, sqlx.Map{
				`reason_id`:   reasonID,
				`reason_type`: dto.ReasonType,
				`reason_name`: r.ReasonName,
				`v_entry`:     true,
				`v_withdraw`:  true,
				`vr_tran`:     true,
				`vr_often`:    true,
				`vr_wh`:       true,
				`vr_snm`:      true,
				`use_flag`:    r.UseFlag,
			})
			dto.ReasonList[i].ReasonID = reasonID
		}
	}

	// Execute
	if _, err = dbs.DH_RETAIL_STORE_W.InsertUpdate(`m_stock_reason`, rReason, []string{`reason_id`}); err != nil {
		return nil, err
	}

	return nil, nil
}

func (d *X_POST_SET_REASON_RQB) validation() error {

	if validx.IsEmpty(d.ReasonType) {
		return errorx.New(`กรุณาระบุ reason_type`).StatusBadRequest()
	}

	d.ReasonType = strings.ToUpper(d.ReasonType)
	if d.ReasonType != `I` && d.ReasonType != `O` {
		return errorx.New(`กรุณาระบุ reason_type ให้ถูกต้อง`).StatusBadRequest()
	}
	for _, v := range d.ReasonList {
		if validx.IsEmpty(v.ReasonName) {
			return errorx.New(`กรุณาระบุ reason_name ให้ถูกต้อง`).StatusBadRequest()
		}
		if validx.IsEmpty(v.UseFlag) {
			return errorx.New(`กรุณาระบุ use_flag ให้ถูกต้อง`).StatusBadRequest()
		}

		if !validx.IsEmpty(v.ReasonID) {
			d.ReasonID = append(d.ReasonID, v.ReasonID)
		}
	}

	return nil
}

func getMaximumReason(reasonType string) (int64, error) {
	rMax, err := dbs.DH_RETAIL_STORE_R.QueryScan(fmt.Sprintf(`select reason_id 
	from m_stock_reason m
	where m.reason_id like '%s%%'
	order by reason_id  desc
	limit 1`, reasonType))
	if err != nil {
		return 0, err
	}
	mMax := rMax.GetRow(0)
	if mMax == nil {
		return 0, nil
	}

	reasonID := mMax.String(`reason_id`)
	if len(reasonID) < 3 {
		return 0, errors.New(`invalid length reason_id`)
	}

	numberReason := tox.Int64(reasonID[1:])
	return numberReason, nil
}
