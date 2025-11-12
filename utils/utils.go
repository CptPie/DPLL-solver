package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

func JSONPrint(input any) {
	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}
	fmt.Println(string(data))

}

func JSONString(input any) string {
	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}
	return string(data)
}
