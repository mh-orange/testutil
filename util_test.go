package testutil

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"
	"time"
)

func TestResult(t *testing.T) {
	result := &Result{}
	if result.Failed() {
		t.Errorf("Expected default result to be success")
	}

	result.Addf("sample message")
	if !result.Failed() {
		t.Errorf("Expected result to be failed")
	}

	if result.String() != "\tsample message" {
		t.Errorf("Got unexpected result string")
	}

	result1 := &Result{}
	result1.Addf("Nested message")
	result.merge(result1)

	if result.String() != "\tsample message\n\tNested message" {
		t.Errorf("Got unexpected result string")
	}
}

func TestMatchType(t *testing.T) {
	time.Local = time.UTC
	t1 := time.Date(2017, time.January, 1, 0, 0, 0, 0, time.UTC)
	t2, _ := t1.MarshalBinary()

	tests := []struct {
		input  interface{}
		match  interface{}
		result bool
	}{
		{int(128), uint(128), true},
		{[]interface{}{int(2), int(4), int(6)}, []uint{2, 4, 6}, true},
		{"foo", 128, false},
		{"2017-01-01T00:00:00-00:00", t1, true},
		{"2017-01-01T00:00:00-00:00", &t1, true},
		{t2, t1, true},
		{t2, &t1, true},
	}

	for i, test := range tests {
		if reflect.DeepEqual(test.input, test.match) {
			t.Errorf("Test %d should have started false!", i)
		}

		test.input = convertType(test.input, test.match)
		if test.result != reflect.DeepEqual(test.input, test.match) {
			if test.result {
				t.Errorf("Test %d %v != %v", i, test.match, test.input)
			} else {
				t.Errorf("Test %d %v == %v", i, test.match, test.input)
			}
		}
	}
}

type bar struct{}

func (b bar) Value5() int { return 5 }

type foo struct {
	b bar
}

func (f foo) Value1() int                   { return 1 }
func (f foo) Value2() int                   { return 2 }
func (f foo) Value3() int                   { return 3 }
func (f foo) Value4() bar                   { return f.b }
func (f foo) Value7() bar                   { return f.b }
func (f foo) GetterWithArg(arg string) bool { return false }
func (f foo) NotGetter()                    {}

func TestGetterFunc(t *testing.T) {
	f := &foo{}
	_, err := getterFunc("bar", f)
	if err == nil {
		t.Errorf("Expected error")
	}

	_, err = getterFunc("GetterWithArg", f)
	if err == nil {
		t.Errorf("Expected error")
	}

	_, err = getterFunc("NotGetter", f)
	if err == nil {
		t.Errorf("Expected error")
	}
}

func TestCompare(t *testing.T) {
	f := &foo{}
	expected := Expected{
		map[string]interface{}{"Value1": 1},
		map[string]interface{}{"Value2": 2},
		map[string]interface{}{"Value3": 3},
		map[string]interface{}{"Value4": Expected{
			map[string]interface{}{"Value5": 5},
		}},
		map[string]interface{}{"Value7": []interface{}{
			map[string]interface{}{"Value5": 5},
		}},
	}

	result := Compare(expected, f)
	if result.Failed() {
		t.Errorf("Expected true")
	}

	expected[2] = map[string]interface{}{"Value3": 4}
	result = Compare(expected, f)
	if !result.Failed() {
		t.Errorf("Expected false")
	}

	expected[2] = map[string]interface{}{"Value3": 3}
	expected = append(expected, map[string]interface{}{"Value6": 3})
	result = Compare(expected, f)
	if !result.Failed() {
		t.Errorf("Expected false")
	}
}

func TestSetBit(t *testing.T) {
	tests := []struct {
		bit   int
		bytes []byte
	}{
		{0, []byte{0x80}},
		{8, []byte{0x00, 0x80}},
		{32, []byte{0x00, 0x00, 0x00, 0x00, 0x80}},
		{34, []byte{0x00, 0x00, 0x00, 0x00, 0x20}},
	}

	for i, test := range tests {
		b := &buffer{}
		b.setBit(test.bit, 1)
		if !bytes.Equal(b.bytes, test.bytes) {
			t.Errorf("Test %d expected %s but got %s", i, hex.Dump(test.bytes), hex.Dump(b.bytes))
		}

		test.bytes = make([]byte, len(test.bytes))
		b.setBit(test.bit, 0)
		if !bytes.Equal(b.bytes, test.bytes) {
			t.Errorf("Test %d expected %s but got %s", i, hex.Dump(test.bytes), hex.Dump(b.bytes))
		}
	}
}

func TestSetBits(t *testing.T) {
	tests := []struct {
		start int
		width uint
		value int
		bytes []byte
	}{
		{0, 2, 3, []byte{0xc0}},
		{2, 12, 0xfff, []byte{0x3f, 0xfc}},
	}

	for i, test := range tests {
		b := &buffer{}
		b.setBits(test.start, test.width, test.value)
		if !bytes.Equal(b.bytes, test.bytes) {
			t.Errorf("Test %d expected\n%s\nbut got\n%s", i, hex.Dump(test.bytes), hex.Dump(b.bytes))
		}
	}
}

func TestCreateByteString(t *testing.T) {
	definitions := []map[string]int{
		{"byte": 0, "bit": 3, "width": 1, "value": 1},
		{"byte": 1, "bit": 4, "width": 4, "value": 15},
		{"byte": 2, "bit": 0, "width": 12, "value": 0xfff},
		{"byte": 4, "width": 16, "value": 0x4242},
	}
	expected := []byte{0x10, 0x0f, 0xff, 0xf0, 0x42, 0x42}

	result := CreateByteString(definitions)
	if !bytes.Equal(result, expected) {
		t.Errorf("Expected\n%s\nGot\n%s\n", hex.Dump(expected), hex.Dump(result))
	}
}

func TestIterateGoodTests(t *testing.T) {
	IterateTests("tests/good.yml", func(name string, test Test) {
		test.Expected[0]["bytes"] = convertType(test.Expected[0]["bytes"], test.Input)
		if !reflect.DeepEqual(test.Input, test.Expected[0]["bytes"]) {
			t.Errorf("Failed to complete test %s: %v does not equal %v", name, test.Input, test.Expected[0]["bytes"])
		}
	})
}

func TestIterateBadTests(t *testing.T) {
	err := IterateTests("tests/bad.yml", func(name string, test Test) {
	})
	if err == nil {
		t.Errorf("Expected an error to be returned")
	}
}
