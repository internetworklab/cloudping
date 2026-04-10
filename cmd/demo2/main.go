package main

import (
	"fmt"
	"log"

	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

func main() {

	path, err := pkgutils.GenerateRandomRGBAPNGBitmap(1200, 1200, (1200-1024)/2)
	if err != nil {
		log.Panic(err)
	}
	fmt.Println(path)
}
