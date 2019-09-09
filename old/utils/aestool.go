package utils

import (
	"encoding/base64"
)

func swap(a, b *byte) {
	c := *a
	*a = *b
	*b = c
}

func MixFour(src []byte, length int) {
	i := 0
	for i < length-3 {
		swap(&src[i+2], &src[i+3])
		swap(&src[i], &src[i+2])
		i += 4
	}
}

func DeMixFour(src []byte, length int) {
	i := 0
	for i < length-3 {
		swap(&src[i], &src[i+2])
		swap(&src[i+2], &src[i+3])
		i += 4
	}
}

func EncryPt(src, dst []byte, length int, extend bool) int {
	if extend {
		for i := 0; i < length%3; i++ {
			src[length] = ' '
			length += 1
		}
	}
	base64.StdEncoding.Encode(dst, src[0:length])
	length = (length + 2) / 3 * 4
	for i := 0; i < length; i++ {
		src[i] = dst[i]
	}
	MixFour(src, length)
	return length
}

func DeCrypt(src, dst []byte, length int) int {
	DeMixFour(src, length)

	base64.StdEncoding.Decode(dst, src[0:length])

	length = length * 3 / 4
	zero := 0

	for i := 0; i < length; i++ { //移除字符中间解码时存在的0
		if dst[i] == 0 {
			zero += 1
		} else {
			src[i-zero] = dst[i]
		}
	}
	return length - zero
}
