package common

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitlab.dohome.technology/dohome-2020/go-servicex/awsx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/excelx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/filex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/sqlx"
	"gitlab.dohome.technology/dohome-2020/go-servicex/timex"
	"gitlab.dohome.technology/dohome-2020/go-servicex/tox"
)

func ExportExcelRpl(reportName string, rows *sqlx.Rows, columns []string) (*string, error) {
	tempFolder, err := filex.CreateTempFolder()
	if err != nil {
		return nil, fmt.Errorf("failed to create temp folder: %v", err)
	}
	defer os.RemoveAll(*tempFolder) // Clean up temp folder when done

	fileName := fmt.Sprintf("%v_%v_%v.xlsx", reportName, time.Now().Local().Format("20060102"), time.Now().Local().Format("150405"))

	pathFile := filepath.Join(tox.String(tempFolder), fileName)

	file := excelx.NewFile(pathFile)
	defer file.Close()

	if err := file.WriteRowH(rows, columns); err != nil {
		return nil, fmt.Errorf("failed to write rows to Excel: %v", err)
	}

	if err := file.Save(); err != nil {
		return nil, fmt.Errorf("failed to save Excel file: %v", err)
	}

	// upload file to s3
	pathS3 := fmt.Sprintf(`reports/retail-store/%v`, time.Now().Local().Format(timex.SYMD))
	fileDto, err := UploadFileToS3(pathFile, pathS3)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file to S3: %v", err)
	}

	// Share file locally
	item, err := awsx.ShareLocal(fileDto.ItemName, fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to share file locally: %v", err)
	}

	return &item.Url, nil
}

func UploadFileToS3(pathFile, pathS3 string) (*awsx.FileDto, error) {
	file, err := os.Open(pathFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Upload file to S3
	fileDto, err := awsx.UploaderTemp(file, pathS3)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file to S3: %v", err)
	}

	return fileDto, nil
}
