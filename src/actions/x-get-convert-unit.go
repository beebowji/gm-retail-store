package actions

import (
	"gitlab.dohome.technology/dohome-2020/go-servicex/gms"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/validx"
	cuarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/cu-article-master"
	gmarticlemaster "gitlab.dohome.technology/dohome-2020/go-structx/gm-article-master"
)

type _X_GET_CONVERT_UNIT struct {
	ArticleId  string  `json:"article_id"`
	UnitCodeFr string  `json:"unit_code_fr"`
	UnitCodeTo string  `json:"unit_code_to"`
	UnitAmtFr  float64 `json:"unit_amt_fr"`
}

type X_CONVERT_UNITS_RSB_WITH_DESC struct {
	Item []X_CONVERT_UNITS_RSB_ITEM_WITH_DESC `json:"item"`
}

type X_CONVERT_UNITS_RSB_ITEM_WITH_DESC struct {
	gmarticlemaster.X_CONVERT_UNITS_RSB_ITEM
	UnitCodeFrDes string `json:"unitCodeFrDes"`
	UnitCodeToDes string `json:"unitCodeToDes"`
}

func XGetConvertUnit(c *gwx.Context) (any, error) {

	var dto _X_GET_CONVERT_UNIT
	if ex := c.ShouldBindJSON(&dto); ex != nil {
		return nil, ex
	}

	// Validate
	if ex := c.Empty(dto.ArticleId, `กรุณาระบุ ArticleId`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(dto.UnitCodeFr, `กรุณาระบุ UnitCodeFr`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(dto.UnitCodeTo, `กรุณาระบุ UnitCodeTo`); ex != nil {
		return nil, ex
	}
	if ex := c.Empty(dto.UnitAmtFr, `กรุณาระบุ CreateBy`); ex != nil {
		return nil, ex
	}

	// Initialize request and response structs
	rqbUnits := gmarticlemaster.X_CONVERT_UNITS_RQB{
		Item: []gmarticlemaster.X_CONVERT_UNITS_RQB_ITEM{
			{
				ArticleId:  dto.ArticleId,
				UnitCodeFr: dto.UnitCodeFr,
				UnitCodeTo: dto.UnitCodeTo,
				UnitAmtFr:  dto.UnitAmtFr,
			},
		},
	}
	rsbUnits := gmarticlemaster.X_CONVERT_UNITS_RSB{}

	// Convert units
	if len(rqbUnits.Item) > 0 {
		if err := gms.GM_ARTICLE_MASTER.HttpPost(`product-units/convert-units`).PayloadJson(rqbUnits).Do().Struct(&rsbUnits); err != nil {
			return nil, err
		}
	}

	// Process response
	rsbUnitsWithDesc := X_CONVERT_UNITS_RSB_WITH_DESC{}
	if len(rsbUnits.Item) > 0 {
		if validx.IsEmpty(rsbUnits.Item[0].ErrorText) {
			unitCodeFrDes := cuarticlemaster.UnitByKey(dto.UnitCodeFr)
			unitCodeToDes := cuarticlemaster.UnitByKey(dto.UnitCodeTo)

			rsbUnitsWithDesc = X_CONVERT_UNITS_RSB_WITH_DESC{
				Item: []X_CONVERT_UNITS_RSB_ITEM_WITH_DESC{
					{
						X_CONVERT_UNITS_RSB_ITEM: rsbUnits.Item[0],
						UnitCodeFrDes:            unitCodeFrDes.String(`name_th`),
						UnitCodeToDes:            unitCodeToDes.String(`name_th`),
					},
				},
			}
		} else {
			return nil, c.Error(rsbUnits.Item[0].ErrorText).StatusBadRequest()
		}
	}

	return rsbUnitsWithDesc, nil
}
