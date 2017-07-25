package testutil

import (
	"encoding"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"gopkg.in/yaml.v2"
)

type Expected []map[string]interface{}

func log(format string, args ...interface{}) {
	//fmt.Printf(format, args...)
}

type Test struct {
	Bits     []map[string]int
	Input    []byte
	Expected Expected
	Err      string `yaml:"error"`
}

type Result struct {
	failures []string
}

func (r *Result) merge(other *Result) {
	for _, failure := range other.failures {
		r.failures = append(r.failures, failure)
	}
}

func (r *Result) Failed() bool {
	return len(r.failures) > 0
}

func (r *Result) Addf(format string, a ...interface{}) {
	r.failures = append(r.failures, fmt.Sprintf(format, a...))
}

func (r *Result) String() string {
	return "\t" + strings.Join(r.failures, "\n\t")
}

func convertType(input interface{}, match interface{}) interface{} {
	log("Converting %T:%v to %T\n", input, input, match)
	receiver := input
	inputValue := reflect.ValueOf(input)
	matchValue := reflect.ValueOf(match)

	inputType := reflect.TypeOf(input)
	matchType := reflect.TypeOf(match)
	var zeroValue reflect.Value
	if matchType.Kind() == reflect.Ptr {
		zeroValue = reflect.New(matchType.Elem())
	} else {
		zeroValue = reflect.New(matchType)
	}
	if inputType.ConvertibleTo(matchType) {
		receiver = inputValue.Convert(matchType).Interface()
	} else if inputType.Kind() == reflect.String && zeroValue.Type().Implements(reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()) {
		zeroValue.MethodByName("UnmarshalText").Call([]reflect.Value{inputValue.Convert(reflect.TypeOf([]byte{}))})
		if matchValue.Kind() == reflect.Ptr {
			receiver = zeroValue.Interface()
		} else {
			receiver = zeroValue.Elem().Interface()
		}
	} else if inputType.Kind() == reflect.Slice {
		if matchType.Kind() == reflect.Slice {
			zeroType := reflect.Zero(matchType.Elem()).Interface()
			receiverArray := reflect.MakeSlice(matchType, 0, 0)
			log("Creating slice of %s\n", matchType)
			for i := 0; i < inputValue.Len(); i++ {
				receiverArray = reflect.Append(receiverArray, reflect.ValueOf(convertType(inputValue.Index(i).Interface(), zeroType)))
			}
			receiver = receiverArray.Interface()
		} else if inputType.Elem().Kind() == reflect.Uint8 && zeroValue.Type().Implements(reflect.TypeOf((*encoding.BinaryUnmarshaler)(nil)).Elem()) {
			zeroValue.MethodByName("UnmarshalBinary").Call([]reflect.Value{inputValue})
			receiver = zeroValue.Interface()

			if matchValue.Kind() == reflect.Ptr {
				receiver = zeroValue.Interface()
			} else {
				receiver = zeroValue.Elem().Interface()
			}
		}
	}
	return receiver
}

func getterFunc(name string, i interface{}) (f reflect.Value, err error) {
	for _, name := range strings.Split(name, ".") {
		object := reflect.ValueOf(i)
		f = object.MethodByName(name)
		if f.Kind() != reflect.Func {
			err = fmt.Errorf("%s is not a method on %v", name, object)
		}

		if err == nil {
			t := f.Type()
			if t.NumIn() == 0 && t.NumOut() == 1 {
				i = f.Call(nil)[0].Interface()
			} else {
				err = fmt.Errorf("%s does not appear to be a getter method", name)
			}
		}
	}
	return f, err
}

func Compare(expectedValues Expected, object interface{}) *Result {
	result := &Result{}

	for _, e := range expectedValues {
		for name, expectedValue := range e {
			log("Comparing %s to %T:%v\n", name, expectedValue, expectedValue)
			f, err := getterFunc(name, object)
			if err == nil {
				output := f.Call(nil)[0].Interface()
				expectedValue = convertType(expectedValue, output)

				if !reflect.DeepEqual(output, expectedValue) {
					result.Addf("%s Expected %v but got %v", name, expectedValue, output)
				}
			} else {
				result.Addf("%v", err)
			}
		}
	}
	return result
}

type buffer struct {
	bytes []byte
}

func (b *buffer) setBit(bit int, value int) {
	byteOffset := bit / 8
	bitOffset := uint(bit % 8)
	if len(b.bytes) <= byteOffset {
		b.bytes = append(b.bytes, make([]byte, 1+byteOffset-len(b.bytes))...)
	}
	if value&0x01 == 0 {
		b.bytes[byteOffset] &= ^(0x01 << (7 - bitOffset))
	} else {
		b.bytes[byteOffset] |= 0x01 << (7 - bitOffset)
	}
}

func (b *buffer) setBits(start int, width uint, value int) {
	value &= ((0x01 << width) - 1)
	for i := uint(0); i < width; i++ {
		b.setBit(start, value>>(width-i-1))
		start++
	}
}

func CreateByteString(bitDefinitions []map[string]int) []byte {
	b := &buffer{}
	for _, bitDefinition := range bitDefinitions {
		bit := 8 * bitDefinition["byte"]
		if bitOffset, ok := bitDefinition["bit"]; ok {
			bit += bitOffset
		}

		width := uint(bitDefinition["width"])
		b.setBits(bit, width, bitDefinition["value"])
	}
	return b.bytes
}

func IterateTests(inputFile string, callback func(name string, test Test)) error {
	tests, err := GetTestData(inputFile)
	if err == nil {
		for name, test := range tests {
			callback(name, *test)
		}
	}
	return err
}

func GetTestData(inputFile string) (map[string]*Test, error) {
	tests := make(map[string]*Test)
	data, err := ioutil.ReadFile(inputFile)
	if err == nil {
		err = yaml.Unmarshal(data, &tests)
	}

	if err == nil {
		for name, test := range tests {
			if len(test.Input) == 0 {
				if len(test.Bits) > 0 {
					test.Input = CreateByteString(test.Bits)
				} else {
					err = fmt.Errorf("Neither input nor bit definitions were specified for test %s", name)
					break
				}
			}
		}
	}
	return tests, err
}
