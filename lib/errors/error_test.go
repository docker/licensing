package errors

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"runtime"
	"testing"

	"github.com/Jeffail/gabs"
	"github.com/stretchr/testify/require"
)

func stackAt(t *testing.T, index int, bits []byte) (file, fname string, line int) {
	dec := json.NewDecoder(bytes.NewReader(bits))
	dec.UseNumber()

	val, err := gabs.ParseJSONDecoder(dec)
	require.NoError(t, err)

	file = val.Path("stack").Index(index).Path("file").Data().(string)
	fname = val.Path("stack").Index(index).Path("func").Data().(string)
	l, err := val.Path("stack").Index(index).Path("line").Data().(json.Number).Int64()
	line = int(l)
	require.NoError(t, err)

	return
}

func baseAt(t *testing.T, name string, bits []byte) (fields Fields, file, fname string, line int, text string) {
	dec := json.NewDecoder(bytes.NewReader(bits))
	dec.UseNumber()

	val, err := gabs.ParseJSONDecoder(dec)
	require.NoError(t, err)

	fields = Fields(val.Path(name).Path("fields").Data().(map[string]interface{}))
	file = val.Path(name).Path("file").Data().(string)
	fname = val.Path(name).Path("func").Data().(string)
	text = val.Path(name).Path("text").Data().(string)
	l, err := val.Path(name).Path("line").Data().(json.Number).Int64()
	line = int(l)
	require.NoError(t, err)

	return
}

func wrapsAt(t *testing.T, index int, bits []byte) (fields Fields, file, fname string, line int, text string) {
	dec := json.NewDecoder(bytes.NewReader(bits))
	dec.UseNumber()

	val, err := gabs.ParseJSONDecoder(dec)
	require.NoError(t, err)

	fields = Fields(val.Path("wraps").Index(index).Path("fields").Data().(map[string]interface{}))
	file = val.Path("wraps").Index(index).Path("file").Data().(string)
	fname = val.Path("wraps").Index(index).Path("func").Data().(string)
	text = val.Path("wraps").Index(index).Path("text").Data().(string)
	l, err := val.Path("wraps").Index(index).Path("line").Data().(json.Number).Int64()
	line = int(l)
	require.NoError(t, err)

	return
}

func TestWrap1(t *testing.T) {
	t.Parallel()

	terr := top1()
	require.Error(t, terr)

	stack, wraps, cause := Cause(terr)
	require.NotNil(t, stack)
	require.Len(t, stack, 5)
	require.NotNil(t, wraps)
	require.NotEqual(t, terr, cause)
	require.Equal(t, "bottom1", cause.Error())
	require.Len(t, wraps, 2)

	m := make(map[string]interface{})
	m["stack"] = stack
	m["wraps"] = wraps
	bits, _ := json.Marshal(&m)

	file, fname, line := stackAt(t, 0, bits)
	require.Equal(t, "error1_test.go", path.Base(file))
	require.Equal(t, "errors.middle1", path.Base(fname))
	require.Equal(t, 17, line)

	file, fname, line = stackAt(t, 1, bits)
	require.Equal(t, "error1_test.go", path.Base(file))
	require.Equal(t, "errors.top1", path.Base(fname))
	require.Equal(t, 6, line)

	fields, file, fname, line, text := wrapsAt(t, 0, bits)
	require.EqualValues(t, "-1", fields["middlenum"])
	require.EqualValues(t, "foo", fields["middleextra"])
	require.Equal(t, "middlestr", fields["middlestr"])
	require.Equal(t, "error1_test.go", path.Base(file))
	require.Equal(t, "errors.middle1", path.Base(fname))
	require.Equal(t, 17, line)
	require.Equal(t, "middle1 wrapf 1", text)

	fields, file, fname, line, text = wrapsAt(t, 1, bits)
	require.EqualValues(t, "1", fields["topnum"])
	require.Equal(t, "topstr", fields["topstr"])
	require.Equal(t, "error1_test.go", path.Base(file))
	require.Equal(t, "errors.top1", path.Base(fname))
	require.Equal(t, 8, line)
	require.Regexp(t, "middle1.*bottom1", path.Base(text))
}

func TestWrap2(t *testing.T) {
	t.Parallel()

	terr := top2()
	require.Error(t, terr)

	stack, wraps, cause := Cause(terr)

	status, ok := HTTPStatus(cause)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, status)

	require.NotNil(t, stack)
	require.Len(t, stack, 6)
	require.NotNil(t, wraps)
	require.NotEqual(t, terr, cause)
	require.Regexp(t, "something not found", cause.Error())
	require.Len(t, wraps, 2)

	m := make(Fields)
	m["stack"] = stack
	m["wraps"] = wraps
	m["cause"] = cause
	bits, _ := json.Marshal(&m)

	fields, file, fname, line, text := baseAt(t, "cause", bits)
	require.EqualValues(t, "1", fields["nffield"])
	require.EqualValues(t, "2", fields["other_nffield"])
	require.Equal(t, "error2_test.go", path.Base(file))
	require.Equal(t, "errors.bottom2", path.Base(fname))
	require.Equal(t, 21, line)
	require.Equal(t, "something not found", text)

	file, fname, line = stackAt(t, 0, bits)
	require.Equal(t, "error2_test.go", path.Base(file))
	require.Equal(t, "errors.bottom2", path.Base(fname))
	require.Equal(t, 21, line)

	file, fname, line = stackAt(t, 1, bits)
	require.Equal(t, "error2_test.go", path.Base(file))
	require.Equal(t, "errors.middle2", path.Base(fname))
	require.Equal(t, 12, line)

	file, fname, line = stackAt(t, 2, bits)
	require.Equal(t, "error2_test.go", path.Base(file))
	require.Equal(t, "errors.top2", path.Base(fname))
	require.Equal(t, 4, line)

	fields, file, fname, line, text = wrapsAt(t, 0, bits)
	require.EqualValues(t, "-1", fields["middlenum"])
	require.Equal(t, "middlestr", fields["middlestr"])
	require.Equal(t, "error2_test.go", path.Base(file))
	require.Equal(t, "errors.middle2", path.Base(fname))
	require.Equal(t, 15, line)
	require.Equal(t, "middle1 wrapf 1", text)

	fields, file, fname, line, text = wrapsAt(t, 1, bits)
	require.EqualValues(t, "1", fields["topnum"])
	require.Equal(t, "topstr", fields["topstr"])
	require.Equal(t, "error2_test.go", path.Base(file))
	require.Equal(t, "errors.top2", path.Base(fname))
	require.Equal(t, 6, line)
	require.Regexp(t, "middle1.*something not found", path.Base(text))
}

func TestPanic1(t *testing.T) {
	// Verifies that there's nothing special about calling stack
	// after recovering from a panic.
	t.Parallel()

	err := panicTop1()
	require.Error(t, err)

	stack, wraps, cause := Cause(err)

	require.NotNil(t, stack)
	// go 1.12 produces 8, go 1.11 and under produces 9
	require.True(t, len(stack) >= 8)
	require.NotNil(t, wraps)
	require.Regexp(t, "panicBottom1", cause.Error())
	require.Len(t, wraps, 1)
}

func makePrettyJSON(bits []byte) string {
	var out bytes.Buffer
	err := json.Indent(&out, bits, "", "\t")
	if err != nil {
		panic(err)
	}
	return out.String()
}

func TestBase(t *testing.T) {
	t.Parallel()

	terr := top3()
	require.Error(t, terr)
	require.Regexp(t, "rock-bottom", terr.Error())

	stack, wraps, cause := Cause(terr)

	_, ok := HTTPStatus(cause)
	require.False(t, ok)

	require.NotNil(t, stack)
	require.Len(t, stack, 6)
	require.NotNil(t, wraps)
	require.NotEqual(t, terr, cause)
	require.Regexp(t, "rock-bottom", cause.Error())
	require.Len(t, wraps, 2)
}

func TestHTTPStatus(t *testing.T) {
	var testerr error
	require.Nil(t, testerr)
	status, ok := HTTPStatus(testerr)
	require.False(t, ok)
	require.Equal(t, http.StatusOK, status)

	testerr = errors.New("unknown error")
	status, ok = HTTPStatus(testerr)
	require.False(t, ok)
	require.Equal(t, http.StatusInternalServerError, status)

	testerr = NewHTTPError(http.StatusPaymentRequired, "insert coins")
	status, ok = HTTPStatus(testerr)
	require.True(t, ok)
	require.Equal(t, http.StatusPaymentRequired, status)
}

func TestNewHTTPError(t *testing.T) {
	err := NewHTTPError(http.StatusTeapot, "boooo")
	require.Len(t, err.Base.CallStack, 3)
	status := err.HTTPStatus()
	require.Equal(t, http.StatusTeapot, status)
}

func BenchmarkCallstack(b *testing.B) {
	for n := 0; n < b.N; n++ {
		var callers [2]uintptr
		ncallers := runtime.Callers(1, callers[:])
		if len(callers) < 1 {
			b.FailNow()
		}

		cframes := runtime.CallersFrames(callers[:ncallers])
		var frames []runtime.Frame
		for {
			frame, more := cframes.Next()
			frames = append(frames, frame)
			if !more {
				break
			}
		}

		if len(frames) < 1 {
			b.FailNow()
		}
	}
}
