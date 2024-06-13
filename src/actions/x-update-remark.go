package actions

import (
	"gitlab.dohome.technology/dohome-2020/go-servicex/dbs"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
)

type _X_UPDATE_REMARK struct {
	Remark string `json:"remark"`
}

func XUpdateRemark(c *gwx.Context) (any, error) {
	id := c.Param("id")
	if id == "" {
		return nil, c.Error("กรุณาระบุ ID").StatusBadRequest()
	}
	var dto _X_UPDATE_REMARK
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}
	dx, ex := sqlx.ConnectPostgresRW(dbs.DH_RETAIL_STORE)
	if ex != nil {
		return nil, ex
	}
	_, err := dx.Exec(`update m_stock set remark = $1 where id = $2`, dto.Remark, id)
	if err != nil {
		return nil, err
	}
	return nil, nil
}
