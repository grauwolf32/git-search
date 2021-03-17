package config

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

// Settings : global settings for the project
var Settings InitStruct

// StartInit : parse config file
func StartInit() {
	parseConfig()
}

func readConfig() []byte {
	var fileData []byte
	file, err := os.Open("./config/Config.json")
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()
	data := make([]byte, 64)

	for {
		n, err := file.Read(data)
		if err == io.EOF {
			break
		}

		fileData = append(fileData, data[:n]...)

	}
	return fileData
}

func parseConfig() {
	b := readConfig()

	err := json.Unmarshal(b, &Settings)
	if err != nil {
		fmt.Println(err)
	}
}
