package utils

// ABogus implements the A-Bogus signature algorithm for Douyin API requests.
// Ported from Python utils/abogus.py. Uses SM3 hashing (github.com/tjfoc/gmsm/sm3).

import (
	"fmt"
	mrand "math/rand"
	"strings"
	"time"

	"github.com/tjfoc/gmsm/sm3"
)

// --- StringProcessor ---

func toCharArray(s string) []int {
	result := make([]int, len(s))
	for i := 0; i < len(s); i++ {
		result[i] = int(s[i])
	}
	return result
}

func toCharStr(arr []int) string {
	b := make([]byte, len(arr))
	for i, v := range arr {
		b[i] = byte(v & 0xFF)
	}
	return string(b)
}

func jsShiftRight(val int64, n uint) int {
	return int((uint32(val)) >> n)
}

func generateRandomBytes(length int) string {
	if length <= 0 {
		length = 3
	}
	var result []byte
	for i := 0; i < length; i++ {
		_rd := mrand.Intn(10000)
		result = append(result,
			byte(((_rd&255)&170)|1),
			byte(((_rd&255)&85)|2),
			byte((jsShiftRight(int64(_rd), 8)&170)|5),
			byte((jsShiftRight(int64(_rd), 8)&85)|40),
		)
	}
	return string(result)
}

// --- CryptoUtility ---

type CryptoUtility struct {
	salt         string
	base64Alpha  []string
	bigArray     []int // mutable copy — transform_bytes modifies in-place
	bigArrayOrig []int // original for reset
}

var defaultBase64Alphabets = []string{
	"Dkdpgh2ZmsQB80/MfvV36XI1R45-WUAlEixNLwoqYTOPuzKFjJnry79HbGcaStCe",
	"ckdp1h4ZKsUB80/Mfvw36XIgR25+WQAlEixNLwoqYTOPuzmFjJnryx9HVGDaStCe",
}

var defaultBigArray = []int{
	121, 243, 55, 234, 103, 36, 47, 228, 30, 231, 106, 6, 115, 95, 78, 101, 250, 207, 198, 50,
	139, 227, 220, 105, 97, 143, 34, 28, 194, 215, 18, 100, 159, 160, 43, 8, 169, 217, 180, 120,
	247, 45, 90, 11, 27, 197, 46, 3, 84, 72, 5, 68, 62, 56, 221, 75, 144, 79, 73, 161,
	178, 81, 64, 187, 134, 117, 186, 118, 16, 241, 130, 71, 89, 147, 122, 129, 65, 40, 88, 150,
	110, 219, 199, 255, 181, 254, 48, 4, 195, 248, 208, 32, 116, 167, 69, 201, 17, 124, 125, 104,
	96, 83, 80, 127, 236, 108, 154, 126, 204, 15, 20, 135, 112, 158, 13, 1, 188, 164, 210, 237,
	222, 98, 212, 77, 253, 42, 170, 202, 26, 22, 29, 182, 251, 10, 173, 152, 58, 138, 54, 141,
	185, 33, 157, 31, 252, 132, 233, 235, 102, 196, 191, 223, 240, 148, 39, 123, 92, 82, 128, 109,
	57, 24, 38, 113, 209, 245, 2, 119, 153, 229, 189, 214, 230, 174, 232, 63, 52, 205, 86, 140,
	66, 175, 111, 171, 246, 133, 238, 193, 99, 60, 74, 91, 225, 51, 76, 37, 145, 211, 166, 151,
	213, 206, 0, 200, 244, 176, 218, 44, 184, 172, 49, 216, 93, 168, 53, 21, 183, 41, 67, 85,
	224, 155, 226, 242, 87, 177, 146, 70, 190, 12, 162, 19, 137, 114, 25, 165, 163, 192, 23, 59,
	9, 94, 179, 107, 35, 7, 142, 131, 239, 203, 149, 136, 61, 249, 14, 156,
}

func NewCryptoUtility(salt string, alphabets []string) *CryptoUtility {
	cu := &CryptoUtility{
		salt:        salt,
		base64Alpha: alphabets,
	}
	cu.bigArray = make([]int, len(defaultBigArray))
	copy(cu.bigArray, defaultBigArray)
	cu.bigArrayOrig = make([]int, len(defaultBigArray))
	copy(cu.bigArrayOrig, defaultBigArray)
	return cu
}

// SM3ToArray computes the SM3 hash of input data and returns it as an int array.
func (cu *CryptoUtility) SM3ToArray(input []byte) []int {
	h := sm3.New()
	h.Write(input)
	hexResult := hexEncode(h.Sum(nil))
	result := make([]int, 0, len(hexResult)/2)
	for i := 0; i+1 < len(hexResult); i += 2 {
		hi := hexLookup[hexResult[i]]
		lo := hexLookup[hexResult[i+1]]
		if hi < 0 || lo < 0 {
			continue
		}
		result = append(result, (hi<<4)|lo)
	}
	return result
}

func hexEncode(b []byte) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, len(b)*2)
	for i, v := range b {
		result[i*2] = hexChars[v>>4]
		result[i*2+1] = hexChars[v&0x0F]
	}
	return string(result)
}

func (cu *CryptoUtility) AddSalt(param string) string {
	return param + cu.salt
}

// ParamsToArray hashes a string param using SM3, optionally adding salt.
func (cu *CryptoUtility) ParamsToArray(param string, addSalt bool) []int {
	if addSalt {
		param = cu.AddSalt(param)
	}
	return cu.SM3ToArray([]byte(param))
}

// TransformBytes performs the custom encryption/decryption on a byte list.
func (cu *CryptoUtility) TransformBytes(bytesList []int) string {
	// Reset bigArray to original state before each call
	copy(cu.bigArray, cu.bigArrayOrig)

	bytesStr := toCharStr(bytesList)
	resultStr := make([]byte, 0, len(bytesStr))
	indexB := cu.bigArray[1]
	initialValue := 0
	valueE := 0

	for index := 0; index < len(bytesStr); index++ {
		if index == 0 {
			initialValue = cu.bigArray[indexB]
			sumInitial := indexB + initialValue
			cu.bigArray[1] = initialValue
			cu.bigArray[indexB] = indexB
			_ = sumInitial // used in next branch's logic
		}

		if index == 0 {
			// Recompute sumInitial for index==0
			sumInitial := indexB + initialValue
			charValue := int(bytesStr[index])
			sumInitialMod := sumInitial % len(cu.bigArray)
			valueF := cu.bigArray[sumInitialMod]
			encryptedChar := charValue ^ valueF
			resultStr = append(resultStr, byte(encryptedChar&0xFF))

			valueE = cu.bigArray[(index+2)%len(cu.bigArray)]
			sumInitial2 := (indexB + valueE) % len(cu.bigArray)
			initialValue = cu.bigArray[sumInitial2]
			cu.bigArray[sumInitial2] = cu.bigArray[(index+2)%len(cu.bigArray)]
			cu.bigArray[(index+2)%len(cu.bigArray)] = initialValue
			indexB = sumInitial2
		} else {
			sumInitial := initialValue + valueE
			charValue := int(bytesStr[index])
			sumInitialMod := sumInitial % len(cu.bigArray)
			valueF := cu.bigArray[sumInitialMod]
			encryptedChar := charValue ^ valueF
			resultStr = append(resultStr, byte(encryptedChar&0xFF))

			valueE = cu.bigArray[(index+2)%len(cu.bigArray)]
			sumInitial2 := (indexB + valueE) % len(cu.bigArray)
			initialValue = cu.bigArray[sumInitial2]
			cu.bigArray[sumInitial2] = cu.bigArray[(index+2)%len(cu.bigArray)]
			cu.bigArray[(index+2)%len(cu.bigArray)] = initialValue
			indexB = sumInitial2
		}
	}

	return string(resultStr)
}

// Base64Encode encodes input using a custom Base64 alphabet.
func (cu *CryptoUtility) Base64Encode(input string, selectedAlphabet int) string {
	// Build binary string from input bytes
	var binaryBuilder strings.Builder
	for i := 0; i < len(input); i++ {
		bits := fmt.Sprintf("%08b", input[i])
		binaryBuilder.WriteString(bits)
	}
	binaryString := binaryBuilder.String()

	// Pad to multiple of 6
	paddingLength := (6 - len(binaryString)%6) % 6
	for i := 0; i < paddingLength; i++ {
		binaryString += "0"
	}

	alphabet := cu.base64Alpha[selectedAlphabet]

	// Convert 6-bit groups to characters
	var output strings.Builder
	for i := 0; i < len(binaryString); i += 6 {
		var idx int
		for j := 0; j < 6; j++ {
			idx = idx<<1 | int(binaryString[i+j]-'0')
		}
		if idx < len(alphabet) {
			output.WriteByte(alphabet[idx])
		}
	}

	// Add padding
	for i := 0; i < paddingLength/2; i++ {
		output.WriteByte('=')
	}

	return output.String()
}

// AbogusEncode encodes using custom Base64 with shifts.
func (cu *CryptoUtility) AbogusEncode(abogusBytesStr string, selectedAlphabet int) string {
	var result []byte
	chars := []byte(abogusBytesStr)
	alphabet := cu.base64Alpha[selectedAlphabet]

	for i := 0; i < len(chars); i += 3 {
		var n int
		if i+2 < len(chars) {
			n = (int(chars[i]) << 16) | (int(chars[i+1]) << 8) | int(chars[i+2])
		} else if i+1 < len(chars) {
			n = (int(chars[i]) << 16) | (int(chars[i+1]) << 8)
		} else {
			n = int(chars[i]) << 16
		}

		shifts := []int{18, 12, 6, 0}
		masks := []int{0xFC0000, 0x03F000, 0x0FC0, 0x3F}
		for j, shift := range shifts {
			if j == 2 && i+1 >= len(chars) {
				break
			}
			if j == 3 && i+2 >= len(chars) {
				break
			}
			idx := (n & masks[j]) >> shift
			if idx < len(alphabet) {
				result = append(result, alphabet[idx])
			}
		}
	}

	// Pad with '=' to multiple of 4
	pad := (4 - len(result)%4) % 4
	for i := 0; i < pad; i++ {
		result = append(result, '=')
	}

	return string(result)
}

// RC4Encrypt encrypts plaintext using RC4.
func (cu *CryptoUtility) RC4Encrypt(key []byte, plaintext string) []byte {
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
	var ciphertext []byte
	for k := 0; k < len(plaintext); k++ {
		i = (i + 1) % 256
		j = (j + s[i]) % 256
		s[i], s[j] = s[j], s[i]
		K := s[(s[i]+s[j])%256]
		ciphertext = append(ciphertext, byte(int(plaintext[k])^K))
	}
	return ciphertext
}

// --- BrowserFingerprintGenerator ---

func GenerateFingerprint(browserType string) string {
	platform := "Win32"
	if browserType == "Safari" {
		platform = "MacIntel"
	}
	return generateFingerprint(platform)
}

func generateFingerprint(platform string) string {
	rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	innerWidth := 1024 + rng.Intn(897)   // 1024-1920
	innerHeight := 768 + rng.Intn(313)   // 768-1080
	outerWidth := innerWidth + 24 + rng.Intn(9) // 24-32
	outerHeight := innerHeight + 75 + rng.Intn(16) // 75-90
	screenX := 0
	screenY := 0
	if rng.Intn(2) == 0 {
		screenY = 30
	}
	sizeWidth := 1024 + rng.Intn(897)
	sizeHeight := 768 + rng.Intn(313)
	availWidth := 1280 + rng.Intn(641)
	availHeight := 800 + rng.Intn(281)

	return fmt.Sprintf("%d|%d|%d|%d|%d|%d|0|0|%d|%d|%d|%d|%d|%d|24|24|%s",
		innerWidth, innerHeight, outerWidth, outerHeight,
		screenX, screenY, sizeWidth, sizeHeight,
		availWidth, availHeight, innerWidth, innerHeight, platform)
}

// --- ABogus ---

var abogusSortIndex = []int{
	18, 20, 52, 26, 30, 34, 58, 38, 40, 53, 42, 21, 27, 54, 55, 31, 35, 57, 39, 41, 43, 22, 28,
	32, 60, 36, 23, 29, 33, 37, 44, 45, 59, 46, 47, 48, 49, 50, 24, 25, 65, 66, 70, 71,
}

var abogusSortIndex2 = []int{
	18, 20, 26, 30, 34, 38, 40, 42, 21, 27, 31, 35, 39, 41, 43, 22, 28, 32, 36, 23, 29, 33, 37,
	44, 45, 46, 47, 48, 49, 50, 24, 25, 52, 53, 54, 55, 57, 58, 59, 60, 65, 66, 70, 71,
}

type ABogus struct {
	aid       int
	pageId    int
	salt      string
	boe       bool
	ddrt      float64
	ic        float64
	paths     []string
	options   []int
	uaKey     []byte
	character string

	crypto    *CryptoUtility
	userAgent string
	browserFP string
}

func NewABogus(fp, userAgent string) *ABogus {
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
			"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0"
	}
	if fp == "" {
		fp = GenerateFingerprint("Edge")
	}

	cu := NewCryptoUtility("cus", defaultBase64Alphabets)

	return &ABogus{
		aid:    6383,
		pageId: 0,
		salt:   "cus",
		boe:    false,
		ddrt:   8.5,
		ic:     8.5,
		paths: []string{
			"^/webcast/", "^/aweme/v1/", "^/aweme/v2/",
			"/v1/message/send", "^/live/", "^/captcha/", "^/ecom/",
		},
		options:   []int{0, 1, 14},
		uaKey:     []byte{0x00, 0x01, 0x0e},
		character: "Dkdpgh2ZmsQB80/MfvV36XI1R45-WUAlEixNLwoqYTOPuzKFjJnry79HbGcaStCe",
		crypto:    cu,
		userAgent: userAgent,
		browserFP: fp,
	}
}

// GenerateAbogus generates the ABogus signature.
// Returns (signedParams, abogus, userAgent, body).
func (a *ABogus) GenerateAbogus(params, body string) (string, string, string, string) {
	abDir := make(map[int]int)
	// Fixed values
	abDir[8] = 3
	abDir[18] = 44
	abDir[19] = 0 // placeholder for [1,0,1,0,1] — used as single int 0 fallback
	abDir[66] = 0
	abDir[69] = 0
	abDir[70] = 0
	abDir[71] = 0

	// Start encryption timestamp
	startEncryption := time.Now().UnixMilli()

	// Params hashing (double hash with salt)
	array1 := a.crypto.ParamsToArray(
		string(bytesFromInts(a.crypto.ParamsToArray(params, true))),
		true,
	)
	array2 := a.crypto.ParamsToArray(
		string(bytesFromInts(a.crypto.ParamsToArray(body, true))),
		true,
	)

	// UA encryption
	uaRc4 := a.crypto.RC4Encrypt(a.uaKey, a.userAgent)
	uaB64 := a.crypto.Base64Encode(string(uaRc4), 1)
	array3 := a.crypto.ParamsToArray(uaB64, false)

	// End encryption timestamp
	endEncryption := time.Now().UnixMilli()

	// Insert start encryption time
	abDir[20] = int((startEncryption >> 24) & 255)
	abDir[21] = int((startEncryption >> 16) & 255)
	abDir[22] = int((startEncryption >> 8) & 255)
	abDir[23] = int(startEncryption & 255)
	abDir[24] = int(uint64(startEncryption) / 256 / 256 / 256 / 256)
	abDir[25] = int(uint64(startEncryption) / 256 / 256 / 256 / 256 / 256)

	// Insert request header config
	abDir[26] = (a.options[0] >> 24) & 255
	abDir[27] = (a.options[0] >> 16) & 255
	abDir[28] = (a.options[0] >> 8) & 255
	abDir[29] = a.options[0] & 255

	// Insert request method
	abDir[30] = (a.options[1] / 256) & 255
	abDir[31] = (a.options[1] % 256) & 255
	abDir[32] = (a.options[1] >> 24) & 255
	abDir[33] = (a.options[1] >> 16) & 255

	// Insert request header encryption
	abDir[34] = (a.options[2] >> 24) & 255
	abDir[35] = (a.options[2] >> 16) & 255
	abDir[36] = (a.options[2] >> 8) & 255
	abDir[37] = a.options[2] & 255

	// Insert request body encryption
	if len(array1) > 22 {
		abDir[38] = array1[21]
		abDir[39] = array1[22]
	}
	if len(array2) > 22 {
		abDir[40] = array2[21]
		abDir[41] = array2[22]
	}
	if len(array3) > 24 {
		abDir[42] = array3[23]
		abDir[43] = array3[24]
	}

	// Insert end encryption time
	abDir[44] = int((endEncryption >> 24) & 255)
	abDir[45] = int((endEncryption >> 16) & 255)
	abDir[46] = int((endEncryption >> 8) & 255)
	abDir[47] = int(endEncryption & 255)
	abDir[48] = abDir[8]
	abDir[49] = int(uint64(endEncryption) / 256 / 256 / 256 / 256)
	abDir[50] = int(uint64(endEncryption) / 256 / 256 / 256 / 256 / 256)

	// Insert fixed values
	abDir[51] = (a.pageId >> 24) & 255
	abDir[52] = (a.pageId >> 16) & 255
	abDir[53] = (a.pageId >> 8) & 255
	abDir[54] = a.pageId & 255
	abDir[55] = a.pageId
	abDir[56] = a.aid
	abDir[57] = a.aid & 255
	abDir[58] = (a.aid >> 8) & 255
	abDir[59] = (a.aid >> 16) & 255
	abDir[60] = (a.aid >> 24) & 255

	// Insert browser fingerprint
	fpLen := len(a.browserFP)
	abDir[64] = fpLen
	abDir[65] = fpLen

	// Get sorted values from abDir using sort_index
	sortedValues := make([]int, 0, len(abogusSortIndex)+fpLen+1)
	for _, idx := range abogusSortIndex {
		sortedValues = append(sortedValues, abDir[idx])
	}

	// Browser fingerprint as ASCII array
	edgeFPArray := toCharArray(a.browserFP)

	// XOR computation using sort_index_2
	abXor := 0
	for i := 0; i < len(abogusSortIndex2)-1; i++ {
		if i == 0 {
			abXor = abDir[abogusSortIndex2[i]]
		}
		abXor ^= abDir[abogusSortIndex2[i+1]]
	}

	// Extend sorted_values with fingerprint array and xor
	for _, v := range edgeFPArray {
		sortedValues = append(sortedValues, v)
	}
	sortedValues = append(sortedValues, abXor)

	// Generate abogus bytes
	abogusBytesStr := generateRandomBytes(3) + a.crypto.TransformBytes(sortedValues)
	abogus := a.crypto.AbogusEncode(abogusBytesStr, 0)

	signedParams := fmt.Sprintf("%s&a_bogus=%s", params, abogus)
	return signedParams, abogus, a.userAgent, body
}

func bytesFromInts(arr []int) []byte {
	b := make([]byte, len(arr))
	for i, v := range arr {
		b[i] = byte(v & 0xFF)
	}
	return b
}
