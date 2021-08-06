package restful

import (
	"strconv"
)

func genSeq(n int64) string {
	if n == 0 {
		n++
	}
	return strconv.FormatInt(n, 10)
}

func nextSeq(seq string) (string, error) {
	n, err := strconv.ParseInt(seq, 10, 64)
	if err != nil {
		return "", err
	}
	n++
	return genSeq(n), nil
}
