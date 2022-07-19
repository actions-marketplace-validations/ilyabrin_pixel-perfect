package main

import "fmt"

const (
	maxImageHeight = 8129 // px
	maxImageWidth  = 8129 // px

	maxFileSize = 20971520 // bytes = "20 Mb"

	minImageHeight = 1 // px
	minImageWidth  = 1 // px

	resultImagePath = "./parts/result.png"

	sizeQuad = 100 // 100px

	defaultMimeType = "png"
)

var (
	errImagesSize = fmt.Errorf("images must be the same size")
)
