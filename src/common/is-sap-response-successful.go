package common

import "gitlab.dohome.technology/dohome-2020/go-structx/sappix"

// Helper function to check if SAP response is successful
func IsSAPResponseSuccessful(resp *sappix.ZDD_HH_PROCESS_ASSIGNLOC_RSB) (bool, string) {
	for _, v := range resp.TBL_LOCATION.Item {
		if v.TYPE != "S" {
			return false, v.MESSAGE
		}
	}
	return true, ""
}
