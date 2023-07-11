package bigcache

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func convertMBToBytes(value int) int {
	return value * 1024 * 1024
}

func isPowerOfTwo(number int) bool {
	return (number != 0) && (number&(number-1)) == 0
}
