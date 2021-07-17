package main

import (
	"fmt"
)

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
	errTooLargeResolutionW = fmt.Errorf("Resolutions is bigger than %d by hight", maxImageHeight)
	errTooLargeResolutionH = fmt.Errorf("Resolutions is bigger than 8129px by width")

	errTooSmallResolutionW = fmt.Errorf("Resolutions is smaller than 1px by hight")
	errTooSmallResolutionH = fmt.Errorf("Resolutions is smaller than 1px by width")
)
