// Copyright 2020-2021 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sql

import (
	"fmt"
	"math"
	"math/bits"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dolthub/vitess/go/sqltypes"
	"github.com/dolthub/vitess/go/vt/proto/query"
	"github.com/shopspring/decimal"
	"gopkg.in/src-d/go-errors.v1"

	"github.com/dolthub/go-mysql-server/sql/encodings"
)

const (
	// SetTypeMaxElements returns the maximum number of elements for the Set type.
	SetTypeMaxElements = 64
)

var (
	ErrConvertingToSet   = errors.NewKind("value %v is not valid for this set")
	ErrDuplicateEntrySet = errors.NewKind("duplicate entry: %v")
	ErrInvalidSetValue   = errors.NewKind("value %v was not found in the set")
	ErrTooLargeForSet    = errors.NewKind(`value "%v" is too large for this set`)

	setValueType = reflect.TypeOf(uint64(0))
)

// Comments with three slashes were taken directly from the linked documentation.

// SetType represents the SET type.
// https://dev.mysql.com/doc/refman/8.0/en/set.html
// The type of the returned value is uint64.
type SetType interface {
	Type
	CharacterSet() CharacterSetID
	Collation() CollationID
	// NumberOfElements returns the number of elements in this set.
	NumberOfElements() uint16
	// BitsToString takes a previously-converted value and returns it as a string.
	BitsToString(bits uint64) (string, error)
	// Values returns all of the set's values in ascending order according to their corresponding bit value.
	Values() []string
}

type setType struct {
	collation             CollationID
	hashedValToBit        map[uint64]uint64
	bitToVal              map[uint64]string
	maxResponseByteLength uint32
}

var _ SetType = setType{}
var _ TypeWithCollation = setType{}

// CreateSetType creates a SetType.
func CreateSetType(values []string, collation CollationID) (SetType, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("number of values may not be zero")
	}
	// A SET column can have a maximum of 64 distinct members.
	if len(values) > SetTypeMaxElements {
		return nil, fmt.Errorf("number of values is too large")
	}

	hashedValToBit := make(map[uint64]uint64)
	bitToVal := make(map[uint64]string)
	var maxByteLength uint32
	maxCharLength := collation.Collation().CharacterSet.MaxLength()
	for i, value := range values {
		// ...SET member values should not themselves contain commas.
		if strings.Contains(value, ",") {
			return nil, fmt.Errorf("values cannot contain a comma")
		}
		if collation != Collation_binary {
			// Trailing spaces are automatically deleted from SET member values in the table definition when a table is created.
			value = strings.TrimRight(value, " ")
		}

		hashedVal, err := collation.HashToUint(value)
		if err != nil {
			return nil, err
		}
		if _, ok := hashedValToBit[hashedVal]; ok {
			return nil, ErrDuplicateEntrySet.New(value)
		}
		bit := uint64(1 << uint64(i))
		hashedValToBit[hashedVal] = bit
		bitToVal[bit] = value
		maxByteLength = maxByteLength + uint32(utf8.RuneCountInString(value)*int(maxCharLength))
		if i != 0 {
			maxByteLength = maxByteLength + uint32(maxCharLength)
		}
	}
	return setType{
		collation:             collation,
		hashedValToBit:        hashedValToBit,
		bitToVal:              bitToVal,
		maxResponseByteLength: maxByteLength,
	}, nil
}

// MustCreateSetType is the same as CreateSetType except it panics on errors.
func MustCreateSetType(values []string, collation CollationID) SetType {
	et, err := CreateSetType(values, collation)
	if err != nil {
		panic(err)
	}
	return et
}

// Compare implements Type interface.
func (t setType) Compare(a interface{}, b interface{}) (int, error) {
	if hasNulls, res := compareNulls(a, b); hasNulls {
		return res, nil
	}

	ai, err := t.Convert(a)
	if err != nil {
		return 0, err
	}
	bi, err := t.Convert(b)
	if err != nil {
		return 0, err
	}
	au := ai.(uint64)
	bu := bi.(uint64)

	if au < bu {
		return -1, nil
	} else if au > bu {
		return 1, nil
	}
	return 0, nil
}

// Convert implements Type interface.
// Returns the string representing the given value if applicable.
func (t setType) Convert(v interface{}) (interface{}, error) {
	if v == nil {
		return nil, nil
	}

	switch value := v.(type) {
	case int:
		return t.Convert(uint64(value))
	case uint:
		return t.Convert(uint64(value))
	case int8:
		return t.Convert(uint64(value))
	case uint8:
		return t.Convert(uint64(value))
	case int16:
		return t.Convert(uint64(value))
	case uint16:
		return t.Convert(uint64(value))
	case int32:
		return t.Convert(uint64(value))
	case uint32:
		return t.Convert(uint64(value))
	case int64:
		return t.Convert(uint64(value))
	case uint64:
		if value <= t.allValuesBitField() {
			return value, nil
		}
	case float32:
		return t.Convert(uint64(value))
	case float64:
		return t.Convert(uint64(value))
	case decimal.Decimal:
		return t.Convert(value.BigInt().Uint64())
	case decimal.NullDecimal:
		if !value.Valid {
			return nil, nil
		}
		return t.Convert(value.Decimal.BigInt().Uint64())
	case string:
		return t.convertStringToBitField(value)
	case []byte:
		return t.Convert(string(value))
	}

	return uint64(0), ErrConvertingToSet.New(v)
}

// MaxTextResponseByteLength implements the Type interface
func (t setType) MaxTextResponseByteLength() uint32 {
	return t.maxResponseByteLength
}

// MustConvert implements the Type interface.
func (t setType) MustConvert(v interface{}) interface{} {
	value, err := t.Convert(v)
	if err != nil {
		panic(err)
	}
	return value
}

// Equals implements the Type interface.
func (t setType) Equals(otherType Type) bool {
	if ot, ok := otherType.(setType); ok && t.collation.Equals(ot.collation) && len(t.bitToVal) == len(ot.bitToVal) {
		for bit, val := range t.bitToVal {
			if ot.bitToVal[bit] != val {
				return false
			}
		}
		return true
	}
	return false
}

// Promote implements the Type interface.
func (t setType) Promote() Type {
	return t
}

// SQL implements Type interface.
func (t setType) SQL(ctx *Context, dest []byte, v interface{}) (sqltypes.Value, error) {
	if v == nil {
		return sqltypes.NULL, nil
	}
	convertedValue, err := t.Convert(v)
	if err != nil {
		return sqltypes.Value{}, err
	}
	value, err := t.BitsToString(convertedValue.(uint64))
	if err != nil {
		return sqltypes.Value{}, err
	}

	resultCharset := ctx.GetCharacterSetResults()
	if resultCharset == CharacterSet_Unspecified || resultCharset == CharacterSet_binary {
		resultCharset = t.collation.CharacterSet()
	}
	encodedBytes, ok := resultCharset.Encoder().Encode(encodings.StringToBytes(value))
	if !ok {
		return sqltypes.Value{}, ErrCharSetFailedToEncode.New(t.collation.CharacterSet().Name())
	}
	val := appendAndSliceBytes(dest, encodedBytes)

	return sqltypes.MakeTrusted(sqltypes.Set, val), nil
}

// String implements Type interface.
func (t setType) String() string {
	s := fmt.Sprintf("set('%v')", strings.Join(t.Values(), `','`))
	if t.CharacterSet() != Collation_Default.CharacterSet() {
		s += " CHARACTER SET " + t.CharacterSet().String()
	}
	if !t.collation.Equals(Collation_Default) {
		s += " COLLATE " + t.collation.String()
	}
	return s
}

// Type implements Type interface.
func (t setType) Type() query.Type {
	return sqltypes.Set
}

// ValueType implements Type interface.
func (t setType) ValueType() reflect.Type {
	return setValueType
}

// Zero implements Type interface.
func (t setType) Zero() interface{} {
	return ""
}

// CharacterSet implements SetType interface.
func (t setType) CharacterSet() CharacterSetID {
	return t.collation.CharacterSet()
}

// Collation implements SetType interface.
func (t setType) Collation() CollationID {
	return t.collation
}

// NumberOfElements implements SetType interface.
func (t setType) NumberOfElements() uint16 {
	return uint16(len(t.hashedValToBit))
}

// BitsToString implements SetType interface.
func (t setType) BitsToString(v uint64) (string, error) {
	return t.convertBitFieldToString(v)
}

// Values implements SetType interface.
func (t setType) Values() []string {
	bitEdge := 64 - bits.LeadingZeros64(t.allValuesBitField())
	valArray := make([]string, bitEdge)
	for i := 0; i < bitEdge; i++ {
		bit := uint64(1 << uint64(i))
		valArray[i] = t.bitToVal[bit]
	}
	return valArray
}

// WithNewCollation implements TypeWithCollation interface.
func (t setType) WithNewCollation(collation CollationID) (Type, error) {
	return CreateSetType(t.Values(), collation)
}

// allValuesBitField returns a bit field that references every value that the set contains.
func (t setType) allValuesBitField() uint64 {
	valCount := uint64(len(t.hashedValToBit))
	if valCount == 64 {
		return math.MaxUint64
	}
	// A set with 3 values will have an upper bound of 8, or 0b1000.
	// 8 - 1 == 7, and 7 is 0b0111, which would map to every value in the set.
	return uint64(1<<valCount) - 1
}

// convertBitFieldToString converts the given bit field into the equivalent comma-delimited string.
func (t setType) convertBitFieldToString(bitField uint64) (string, error) {
	strBuilder := strings.Builder{}
	bitEdge := 64 - bits.LeadingZeros64(bitField)
	writeCommas := false
	if bitEdge > len(t.bitToVal) {
		return "", ErrTooLargeForSet.New(bitField)
	}
	for i := 0; i < bitEdge; i++ {
		bit := uint64(1 << uint64(i))
		if bit&bitField != 0 {
			val, ok := t.bitToVal[bit]
			if !ok {
				return "", ErrInvalidSetValue.New(bitField)
			}
			if writeCommas {
				strBuilder.WriteByte(',')
			} else {
				writeCommas = true
			}
			strBuilder.WriteString(val)
		}
	}
	return strBuilder.String(), nil
}

// convertStringToBitField converts the given string into a bit field.
func (t setType) convertStringToBitField(str string) (uint64, error) {
	if str == "" {
		return 0, nil
	}
	var bitField uint64
	vals := strings.Split(str, ",")
	for _, val := range vals {
		compareVal := val
		if t.collation != Collation_binary {
			compareVal = strings.TrimRight(compareVal, " ")
		}
		hashedVal, err := t.collation.HashToUint(compareVal)
		if err == nil {
			if bit, ok := t.hashedValToBit[hashedVal]; ok {
				bitField |= bit
				continue
			}
		}

		asUint, err := strconv.ParseUint(val, 10, 64)
		if err == nil {
			if asUint == 0 {
				continue
			}
			if _, ok := t.bitToVal[asUint]; ok {
				bitField |= asUint
				continue
			}
		}
		return 0, ErrInvalidSetValue.New(val)
	}
	return bitField, nil
}
