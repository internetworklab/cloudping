package main

import (
	"fmt"

	pkgutils "github.com/internetworklab/cloudping/pkg/utils"
)

func main() {
	w := 1200
	path := pkgutils.GenerateRandomRGBAPNGBitmap(w, w)
	fmt.Println(path)
}
