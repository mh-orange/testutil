package testutil

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"gopkg.in/yaml.v2"
)

type Expected []map[string]interface{}
type Test struct {
	Bits   []map[string]int
	Input  []byte
	Output Expected
	Err    string `yaml:"error"`
}

type Result struct {
	failures []string
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

func matchType(i interface{}, match interface{}) interface{} {
	value := reflect.ValueOf(i)
	matchValue := reflect.ValueOf(match)

	if value.Kind() == reflect.Slice && matchValue.Kind() == reflect.Slice {
		t := matchValue.Type().Elem()
		matchZeroValue := reflect.Zero(t).Interface()
		if inputArray, ok := i.([]interface{}); ok {
			outputArray := reflect.MakeSlice(reflect.TypeOf(match), 0, 0)
			for i := 0; i < len(inputArray); i++ {
				outputArray = reflect.Append(outputArray, reflect.ValueOf(matchType(inputArray[i], matchZeroValue)))
			}
			return outputArray.Interface()
		}
	}

	t := reflect.TypeOf(i)
	if t.ConvertibleTo(reflect.TypeOf(match)) {
		return value.Convert(reflect.TypeOf(match)).Interface()
	}
	return i
}

func getterFunc(name string, i interface{}) (f reflect.Value, err error) {
	object := reflect.ValueOf(i)

	f = object.MethodByName(name)
	if f.Kind() != reflect.Func {
		err = fmt.Errorf("%s is not a method on %v", name, object)
	}

	if err == nil {
		t := f.Type()
		if t.NumIn() != 0 || t.NumOut() == 0 {
			err = fmt.Errorf("%s does not appear to be a getter method", name)
		}
	}
	return f, err
}

func Compare(expectedValues Expected, object interface{}) *Result {
	result := &Result{}

	for _, e := range expectedValues {
		for name, expectedValue := range e {
			f, err := getterFunc(name, object)
			if err == nil {
				output := f.Call(nil)[0].Interface()

				expectedValue = matchType(expectedValue, output)

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
	for i := width - 1; ; i-- {
		b.setBit(start, value>>(width-i-1))
		start++
		if i == 0 {
			break
		}
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
