package runners

import (
	"gitlab.dohome.technology/dohome-2020/go-servicex/config"
	"gitlab.dohome.technology/dohome-2020/go-servicex/kafkax"
)

func GetRunners() map[string]map[string]kafkax.WK {

	var ServiceName = config.GetServiceName()

	var RUNNERS = map[string]map[string]kafkax.WK{
		ServiceName: {
			// paths.TEST_CONS: {W: poscar.PosCarCons, T: topics.POS_CAR},
		},
	}

	return RUNNERS
}
