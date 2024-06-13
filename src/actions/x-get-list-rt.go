package actions

import (
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	cuarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/cu-article-master"
)

type GetListRtRSB struct {
	RtList []rtList `json:"rt_list"`
}

type rtList struct {
	SellerCode string `json:"seller_code"`
	SellerName string `json:"seller_name"`
}

// ดึง RT ผู้ดูแลขาย
func XGetListRt(c *gwx.Context) (any, error) {
	// Load cache
	cacheRT, err := cuarticlemaster.SalesRepresentative()
	if err != nil {
		return nil, err
	}

	response := &GetListRtRSB{RtList: []rtList{}}
	for _, v := range cacheRT.Rows {
		response.RtList = append(response.RtList, rtList{
			SellerCode: v.String(`seller_code`),
			SellerName: v.String(`seller_name`),
		})
	}
	return response, nil
}
