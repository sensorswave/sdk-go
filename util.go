package sensorswave

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewUUID returns a new UUID (v7 by default, fallback to v4).
func NewUUID() string {
	idv7, err := uuid.NewV7()
	if err != nil { // should never happen
		return uuid.NewString() // uuid v4
	} else {
		return idv7.String()
	}
}

type BucketBitmap struct {
	data []byte
}

func NewBucketBitmap(bits int) BucketBitmap {
	// 1000 bits = 125 bytes
	// 1001 bits = 126 bytes
	// 1007 bits = 126 bytes
	length := (bits + 7) / 8

	bm := BucketBitmap{}
	bm.data = make([]byte, length)

	return bm
}

// SaveNetworkByteOrderString saves the bitmap as a big-endian hex string
func (bm *BucketBitmap) SaveNetworkByteOrderString() string {
	return hex.EncodeToString(bm.data[:])
}

// LoadNetworkByteOrderString loads the bitmap from a big-endian hex string
func (bm *BucketBitmap) LoadNetworkByteOrderString(encoded string) error {
	bytes, err := hex.DecodeString(encoded)
	if err != nil {
		return err
	}
	if len(bytes) > len(bm.data) {
		copy(bm.data[:], bytes[:len(bm.data)])
	} else {
		copy(bm.data[:], bytes)
	}

	return nil
}

// SetBit sets the bit at the specified position in the bitmap to 1
func (bm *BucketBitmap) SetBit(pos int) {
	if pos < 0 || pos >= len(bm.data)*8 {
		return
	}
	byteIndex := pos / 8
	bitIndex := pos % 8
	bm.data[byteIndex] |= (1 << (7 - bitIndex)) // Big-endian logic: high bit first
}

// ClearBit clears the bit at the specified position (sets to 0)
func (bm *BucketBitmap) ClearBit(pos int) {
	if pos < 0 || pos >= len(bm.data)*8 {
		return
	}
	byteIndex := pos / 8
	bitIndex := pos % 8
	bm.data[byteIndex] &^= (1 << (7 - bitIndex)) // Big-endian logic: high bit first
}

// GetBit gets the bit value at the specified position
func (bm *BucketBitmap) GetBit(pos int) int {
	if pos < 0 || pos >= len(bm.data)*8 {
		return 0
	}
	byteIndex := pos / 8
	bitIndex := pos % 8
	bit := (bm.data[byteIndex] >> (7 - bitIndex)) & 1 // Big-endian logic: high bit first
	return int(bit)
}

// Count counts the number of bits set to 1
func (bm *BucketBitmap) Count() int {
	cnt := 0
	for i := 0; i < len(bm.data); i++ {
		for j := 0; j < 8; j++ {
			if bm.data[i]&(1<<(7-j)) != 0 { // Big-endian logic: high bit first
				cnt++
			}
		}
	}

	return cnt
}

// ///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// hash calculates the SHA-256 hash value of a string
func hash(key string) []byte {
	hasher := sha256.New()
	bytes := []byte(key)
	hasher.Write(bytes)
	return hasher.Sum(nil)
}

func hashUint64(key string, salt string) uint64 {
	hash := hash(fmt.Sprintf("%s.%s", key, salt))
	return binary.BigEndian.Uint64(hash)
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func compareNumbers(a, b interface{}, fun func(x, y float64) bool) bool {
	numA, okA := getNumericValue(a)
	numB, okB := getNumericValue(b)
	if !okA || !okB {
		return false
	}
	return fun(numA, numB)
}

func getNumericValue(a interface{}) (float64, bool) {
	if a == nil {
		return 0, false
	}
	aVal := reflect.ValueOf(a)
	switch reflect.TypeOf(a).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(aVal.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(aVal.Uint()), true
	case reflect.Float32, reflect.Float64:
		return float64(aVal.Float()), true
	case reflect.String:
		f, err := strconv.ParseFloat(aVal.String(), 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

func splitVersionString(version string) ([]int64, error) {
	stringParts := strings.Split(version, ".")
	numParts := make([]int64, len(stringParts))
	for i := range stringParts {
		n1, e := strconv.ParseInt(stringParts[i], 10, 64)
		if e != nil {
			return numParts, e
		}
		numParts[i] = n1
	}
	return numParts, nil
}

func compareVersionsSlice(v1 []int64, v2 []int64) int {
	i := 0
	v1len := len(v1)
	v2len := len(v2)
	for i < maxInt(v1len, v2len) {
		var n1 int64
		if i >= v1len {
			n1 = 0
		} else {
			n1 = v1[i]
		}
		var n2 int64
		if i >= v2len {
			n2 = 0
		} else {
			n2 = v2[i]
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
		i++
	}
	return 0
}

func compareVersions(a, b interface{}, fun func(x, y []int64) bool) bool {
	strA, okA := a.(string)
	strB, okB := b.(string)
	if !okA || !okB {
		return false
	}
	v1 := strings.Split(strA, "-")[0]
	v2 := strings.Split(strB, "-")[0]
	if len(v1) == 0 || len(v2) == 0 {
		return false
	}

	v1Versions, e1 := splitVersionString(v1)
	v2Versions, e2 := splitVersionString(v2)
	if e1 != nil || e2 != nil {
		return false
	}
	return fun(v1Versions, v2Versions)
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func arrayAny(val interface{}, arr interface{}, fun func(x, y interface{}) bool) bool {
	if array, ok := arr.([]interface{}); ok {
		for _, arrVal := range array {
			if fun(val, arrVal) {
				return true
			}
		}
	}
	return false
}

func compareStrings(s1 interface{}, s2 interface{}, ignoreCase bool, fun func(x, y string) bool) bool {
	var str1, str2 string
	if s1 == nil || s2 == nil {
		return false
	}
	str1 = convertToString(s1)
	str2 = convertToString(s2)

	if ignoreCase {
		return fun(strings.ToLower(str1), strings.ToLower(str2))
	}
	return fun(str1, str2)
}

func convertToString(a interface{}) string {
	if a == nil {
		return ""
	}
	if asString, ok := a.(string); ok {
		return asString
	}
	aVal := reflect.ValueOf(a)
	switch aVal.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(aVal.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(aVal.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(aVal.Float(), 'f', -1, 64)
	case reflect.Bool:
		return strconv.FormatBool(aVal.Bool())
	case reflect.String:
		return fmt.Sprintf("%v", a)
	case reflect.Slice, reflect.Array:
		var result []string
		for i := 0; i < aVal.Len(); i++ {
			result = append(result, fmt.Sprintf("%v", aVal.Index(i).Interface()))
		}
		return strings.Join(result, ",")
	}

	return fmt.Sprintf("%v", a)
}

func getTime(a interface{}) time.Time {
	switch v := a.(type) {
	case float64, int64, int32, int:
		tSec := time.Unix(getUnixTimestamp(v), 0)
		if tSec.Year() > time.Now().Year()+100 {
			return time.Unix(getUnixTimestamp(v)/1000, 0)
		}
		return tSec
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			return t
		}
		vInt, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return time.Time{}
		}
		tSec := time.Unix(getUnixTimestamp(vInt), 0)
		if tSec.Year() > time.Now().Year()+100 {
			return time.Unix(getUnixTimestamp(vInt)/1000, 0)
		}
		return tSec
	}
	return time.Time{}
}

func getUnixTimestamp(v interface{}) int64 {
	switch v := v.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	}
	return 0
}

func deepEqual(left any, right any) bool {
	equal := false
	if right == nil {
		equal = left == nil || left == ""
	} else {
		equal = reflect.DeepEqual(left, right)
	}
	return equal
}
