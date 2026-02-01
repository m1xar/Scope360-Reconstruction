package reconstructor

import (
	"strconv"
	"strings"
)

func mustFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func isOpen(dir string) bool {
	return dir == "Open Long" || dir == "Open Short"
}

func isClose(dir string) bool {
	return dir == "Close Long" || dir == "Close Short"
}

func isPerpDir(dir string) bool {
	return isOpen(dir) || isClose(dir)
}

func sideFromDir(dir string) string {
	if strings.Contains(dir, "Long") {
		return "Long"
	}
	return "Short"
}
