package routex

import (
	"gitlab.dohome.technology/dohome-2020/gm-retail-store/src/actions"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gms"
	"gitlab.dohome.technology/dohome-2020/go-servicex/gwx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/jwtx"
)

func Routex() {

	// // https://hoohoo.top/blog/20220320172715-go-websocket/
	// hub := wss.CreateHub()
	// go hub.Run()

	// connect
	gms.GM_RETAIL_STORE.Connect(func(g *gwx.GX) {
		rGuest := g.Group(``)
		{
			// g.GET(rGuest, `/actions/list-reason`, actions.XListReason)
			// g.POST(rGuest, `/actions/submit-reason`, actions.XSubmitReason)
			g.POST(rGuest, `actions/submit-goods-receipt`, actions.XSubmitGoodsReceipt)
			g.GET(rGuest, `actions/get-detail-reprint/:req_no`, actions.XGetDetailReprint)
		}
		rAuth := g.Group(``, jwtx.Guard())
		{
			rAction := rAuth.Group(`actions`)
			{
				g.GET(rAction, `get-list-rt`, actions.XGetListRt)
				g.GET(rAction, `get-list-reason`, actions.XGetListReasonV2)
				g.GET(rAction, `get-is-serial`, actions.XGetIsSerial)
				g.GET(rAction, `get-binlocation`, actions.XGetBinlocation)
				g.GET(rAction, `get-list-mc-level2`, actions.XGetListMcLevel2)
				g.GET(rAction, `get-list-binlocation-master`, actions.XGetListBinLocationMaster)
				g.GET(rAction, `get-remainlift-article`, actions.XGetRemainlifeArticle)
				g.GET(rAction, `get-history-recv`, actions.XGetHistoryRecv)
				g.GET(rAction, `get-convert-unit`, actions.XGetConvertUnit)

				g.POST(rAction, `submit-reason`, actions.XSubmitReason)
				g.POST(rAction, `post-article-isbatch`, actions.XPostArticleIsbatch)
				g.POST(rAction, `post-wd-stock`, actions.XPostWdStock)
				g.POST(rAction, `post-set-reason`, actions.XPostSetReason)
				g.POST(rAction, `post-article-instock`, actions.XPostArticleInstock)
				g.POST(rAction, `post-switch-binlocation`, actions.XPostSwitchBinLocation)
				g.POST(rAction, `post-move-article-all`, actions.XPostMoveArticleAll)
				g.POST(rAction, `post-move-article-id`, actions.XPostMoveArticleId)
				g.POST(rAction, `post-recv-stock`, actions.XPostRecvStock)
				g.POST(rAction, `post-set-binlocation`, actions.XPostSetBinlocation)
				g.POST(rAction, `post-reset-binlocation`, actions.XPostResetBinlocation)
				g.POST(rAction, `post-list-report-req`, actions.XPostListReportReq)
				g.POST(rAction, `post-list-state-binlocation`, actions.XPostListStateBinlocation)
				g.POST(rAction, `post-detail-article`, actions.XPostDetailArticle)
				g.POST(rAction, `post-list-article-action`, actions.XPostListArticleAction)
				g.POST(rAction, `post-list-wd-often`, actions.XPostListWdOften)
				g.POST(rAction, `post-list-article-instock`, actions.XPostListArticleInstock)
				g.POST(rAction, `post-list-deadstock`, actions.XPostListDeadstock)
				g.PUT(rAction, `update-stock-remark/:id`, actions.XUpdateRemark)
				g.GET(rAction, `get-barcode-scan/:barcode`, actions.XGetBarcode)

			}
		}
	})

}
