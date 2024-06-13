.PHONY: getall
getall: 
	@ go get -u gitlab.dohome.technology/dohome-2020/go-servicex
	@ go get -u gitlab.dohome.technology/dohome-2020/go-structx
	@ go mod tidy 
