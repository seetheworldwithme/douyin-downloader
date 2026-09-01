package utils

// XBogus implements the X-Bogus signature algorithm for Douyin API requests.
// Ported byte-for-byte from Python utils/xbogus.py.

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

var hexLookup [128]int

// Initialize hex lookup table (-1 = invalid, 0-15 = hex value)
func init() {
	for i := range hexLookup {
		hexLookup[i] = -1
	}
	for i := 0; i <= 9; i++ {
		hexLookup['0'+i] = i
	}
	for i := 0; i <= 5; i++ {
		hexLookup['a'+i] = 10 + i
		hexLookup['A'+i] = 10 + i
	}
}

const xbogusCharacter = "Dkdpgh4ZKsQB80/Mfvw36XI1R25-WUAlEi7NLboqYTOPuzmFjJnryx9HVGcaStCe="

var defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36"

var uaKey = []byte{0x00, 0x01, 0x0c}

// XBogus generates X-Bogus signatures.
type XBogus struct {
	userAgent string
}

// NewXBogus creates an XBogus signer with the given User-Agent.
func NewXBogus(userAgent string) *XBogus {
	if userAgent == "" {
		userAgent = defaultUA
	}
	return &XBogus{userAgent: userAgent}
}

// md5HexStrToArray converts an MD5 hex string to a byte array.
// If the string is longer than 32 chars, it returns the raw byte values.
func md5HexStrToArray(md5Str string) []int {
	if len(md5Str) > 32 {
		result := make([]int, len(md5Str))
		for i := 0; i < len(md5Str); i++ {
			result[i] = int(md5Str[i])
		}
		return result
	}

	array := make([]int, 0, len(md5Str)/2)
	for i := 0; i+1 < len(md5Str); i += 2 {
		hi := hexLookup[md5Str[i]]
		lo := hexLookup[md5Str[i+1]]
		if hi < 0 || lo < 0 {
			continue
		}
		array = append(array, (hi<<4)|lo)
	}
	return array
}

func md5Hash(input []int) string {
	bytes := make([]byte, len(input))
	for i, v := range input {
		bytes[i] = byte(v & 0xFF)
	}
	h := md5.Sum(bytes)
	return hex.EncodeToString(h[:])
}

func md5HashStr(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

func md5Encrypt(urlPath string) []int {
	first := md5HashStr(urlPath)
	hashed := md5Hash(md5HexStrToArray(first))
	return md5HexStrToArray(hashed)
}

func encodingConversion(a, b, c, e, d, t, f, r, n, o, i, under, x, u, s, l, v, h, p int) []byte {
	payload := []int{
		a, i, b, under, c, x, e, u, d, s, t, l, f, v, r, h, n, p, o,
	}
	result := make([]byte, len(payload))
	for i, val := range payload {
		result[i] = byte(val & 0xFF)
	}
	return result
}

func rc4Encrypt(key, data []byte) []byte {
	s := make([]int, 256)
	for i := range s {
		s[i] = i
	}

	j := 0
	for i := 0; i < 256; i++ {
		j = (j + s[i] + int(key[i%len(key)])) % 256
		s[i], s[j] = s[j], s[i]
	}

	i := 0
	j = 0
	encrypted := make([]byte, len(data))
	for k, b := range data {
		i = (i + 1) % 256
		j = (j + s[i]) % 256
		s[i], s[j] = s[j], s[i]
		encrypted[k] = b ^ byte(s[(s[i]+s[j])%256])
	}
	return encrypted
}

func calculation(a1, a2, a3 int) string {
	x3 := ((a1 & 255) << 16) | ((a2 & 255) << 8) | (a3 & 255)
	return string([]byte{
		xbogusCharacter[(x3&16515072)>>18],
		xbogusCharacter[(x3&258048)>>12],
		xbogusCharacter[(x3&4032)>>6],
		xbogusCharacter[x3&63],
	})
}

// Build generates the X-Bogus signed URL.
// Returns (signedURL, xbogus, userAgent).
func (x *XBogus) Build(rawURL string) (string, string, string) {
	uaBytes := []byte(x.userAgent)
	// Encode UA through ISO-8859-1 lens (Latin-1, bytes as-is for ASCII UAs)
	uaRc4 := rc4Encrypt(uaKey, uaBytes)
	uaB64 := base64.StdEncoding.EncodeToString(uaRc4)
	// Use base64 string through ISO-8859-1 char lens
	uaB64Bytes := make([]byte, len(uaB64))
	for i := 0; i < len(uaB64); i++ {
		uaB64Bytes[i] = uaB64[i] // ASCII = same in Latin1
	}

	uaMd5Array := md5HexStrToArray(md5HashStr(string(uaB64Bytes)))
	emptyMd5Array := md5HexStrToArray(md5Hash(md5HexStrToArray("d41d8cd98f00b204e9800998ecf8427e")))
	urlMd5Array := md5Encrypt(rawURL)

	timer := int(time.Now().Unix())
	ct := 536919696

	newArray := []int{
		64, 0, 1, 12,
		urlMd5Array[14], urlMd5Array[15],
		emptyMd5Array[14], emptyMd5Array[15],
		uaMd5Array[14], uaMd5Array[15],
		(timer >> 24) & 255, (timer >> 16) & 255, (timer >> 8) & 255, timer & 255,
		(ct >> 24) & 255, (ct >> 16) & 255, (ct >> 8) & 255, ct & 255,
	}

	xorResult := newArray[0]
	for _, val := range newArray[1:] {
		xorResult ^= val
	}
	newArray = append(newArray, xorResult)

	// Split into odd and even indexed arrays
	var array3, array4 []int
	for idx := 0; idx < len(newArray); idx += 2 {
		array3 = append(array3, newArray[idx])
		if idx+1 < len(newArray) {
			array4 = append(array4, newArray[idx+1])
		}
	}
	merged := append(array3, array4...)

	// encoding_conversion with 19 params
	ecBytes := encodingConversion(
		merged[0],  // a
		merged[1],  // b
		merged[2],  // c
		merged[3],  // e
		merged[4],  // d
		merged[5],  // t
		merged[6],  // f
		merged[7],  // r
		merged[8],  // n
		merged[9],  // o
		merged[10], // i
		merged[11], // under
		merged[12], // x
		merged[13], // u
		merged[14], // s
		merged[15], // l
		merged[16], // v
		merged[17], // h
		merged[18], // p
	)

	// RC4 with key 0xFF (ÿ in ISO-8859-1)
	rc4Key := []byte{0xFF}
	rc4Data := rc4Encrypt(rc4Key, ecBytes)

	// Prepend chr(2) and chr(255)
	garbled := make([]byte, 0, 2+len(rc4Data))
	garbled = append(garbled, 2, 255)
	garbled = append(garbled, rc4Data...)

	// Calculate X-Bogus
	xb := ""
	for i := 0; i+2 < len(garbled); i += 3 {
		xb += calculation(int(garbled[i]), int(garbled[i+1]), int(garbled[i+2]))
	}

	signedURL := fmt.Sprintf("%s&X-Bogus=%s", rawURL, xb)
	return signedURL, xb, x.userAgent
}
