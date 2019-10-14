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

func EncryPt(src, dst []byte, length int) int { // length % 3 === 0
	base64.StdEncoding.Encode(dst, src[0:length])
	length = length / 3
	length *= 4
	MixFour(dst, length)
	return length
}

func DeCrypt(src, dst []byte, length int) int { // length % 4 === 0
	DeMixFour(src, length)
	length, _ = base64.StdEncoding.Decode(dst, src[0:length])
	//对面可能传来的是含 ==字符串，即不为3倍数的字符串加密,故以返回长度为准
	return length
}
