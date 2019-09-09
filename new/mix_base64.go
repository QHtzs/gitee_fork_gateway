package main

/*
变形的base64加密解密
*/

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

func EncryPt(src, dst []byte, length int) int {
	base64.StdEncoding.Encode(dst, src[0:length])
	length = (length + 2) / 3
	length *= 4
	MixFour(dst, length)
	return length
}

func DeCrypt(src, dst []byte, length int) int {
	DeMixFour(src, length)
	base64.StdEncoding.Decode(dst, src[0:length])
	length /= 4
	length *= 3
	for i := length - 1; i >= 0; i-- {
		if dst[i] == 0 {
			length -= 1
			for j := i; j < length; j++ {
				dst[j] = dst[j+1]
			}
		}
	}
	return length
}
