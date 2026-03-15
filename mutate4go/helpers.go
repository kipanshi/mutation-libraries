package mutate4go

import "strconv"

func intToString(value int) string {
	return strconv.Itoa(value)
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
