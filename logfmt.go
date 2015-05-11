// Package logfmt implements utilities to marshal and unmarshal data in the
// logfmt format. The logfmt format records key/value pairs in a way that
// balances readability for humans and simplicity of computer parsing. It is
// most commonly used as a more human friendly alternative to JSON for
// structured logging.
package logfmt

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

// MarshalKeyvals returns the logfmt encoding of keyvals, a variadic sequence
// of alternating keys and values.
func MarshalKeyvals(keyvals ...interface{}) ([]byte, error) {
	if len(keyvals) == 0 {
		return nil, nil
	}
	if len(keyvals)%2 == 1 {
		keyvals = append(keyvals, nil)
	}
	buf := &bytes.Buffer{}
	enc := NewEncoder(buf)
	var err error
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			err = enc.EncodeKeyval(keyvals[i], keyvals[i+1])
		} else {
			err = enc.EncodeKeyval(keyvals[i], nil)
		}
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// An Encoder writes logfmt data to an output stream.
type Encoder struct {
	w       io.Writer
	needSep bool
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w: w,
	}
}

var (
	space    = []byte(" ")
	equals   = []byte("=")
	newline  = []byte("\n")
	nilbytes = []byte("nil")
)

// ErrNilKey is returned by Marshal functions and Encoder methods if a key is
// a nil interface or pointer value.
var ErrNilKey = errors.New("nil key")

// ErrInvalidKey is returned by Marshal functions and Encoder methods if a key
// contains an invalid character.
var ErrInvalidKey = errors.New("invalid key")

// EncodeKeyval writes the logfmt encoding of key and value to the stream. A
// single space is written before the second and subsequent keys in a record.
func (enc *Encoder) EncodeKeyval(key, value interface{}) error {
	if key == nil {
		return ErrNilKey
	}
	rkey := reflect.ValueOf(key)
	if rkey.Kind() == reflect.Ptr && rkey.IsNil() {
		return ErrNilKey
	}
	if enc.needSep {
		if _, err := enc.w.Write(space); err != nil {
			return err
		}
	} else {
		enc.needSep = true
	}
	if err := enc.writeKey(key); err != nil {
		return err
	}
	if _, err := enc.w.Write(equals); err != nil {
		return err
	}
	return enc.writeValue(value)
}

func (enc *Encoder) writeKey(kv interface{}) (err error) {
	switch v := kv.(type) {
	case string:
		if strings.ContainsAny(v, ` "=`) {
			return ErrInvalidKey
		}
		_, err = io.WriteString(enc.w, v)
	case encoding.TextMarshaler:
		defer func() {
			if recErr := recover(); recErr != nil {
				if v := reflect.ValueOf(kv); v.Kind() == reflect.Ptr && v.IsNil() {
					_, err = enc.w.Write(nilbytes)
				} else {
					panic(recErr)
				}
			}
		}()
		var b []byte
		b, err = v.MarshalText()
		if err != nil {
			return
		}
		if bytes.IndexAny(b, ` "=`) >= 0 {
			return ErrInvalidKey
		}
		_, err = enc.w.Write(b)
	default:
		str := fmt.Sprint(v)
		if strings.ContainsAny(str, ` "=`) {
			return ErrInvalidKey
		}
		_, err = io.WriteString(enc.w, str)
	}
	return err
}

func (enc *Encoder) writeValue(kv interface{}) (err error) {
	switch v := kv.(type) {
	case nil:
		enc.w.Write(nilbytes)
	case string:
		if strings.ContainsAny(v, ` "=`) {
			v = strconv.Quote(v)
		}
		if v == "nil" {
			v = `"nil"`
		}
		_, err = io.WriteString(enc.w, v)
	case encoding.TextMarshaler:
		defer func() {
			if recErr := recover(); recErr != nil {
				if v := reflect.ValueOf(kv); v.Kind() == reflect.Ptr && v.IsNil() {
					_, err = enc.w.Write(nilbytes)
				} else {
					panic(recErr)
				}
			}
		}()
		var b []byte
		b, err = v.MarshalText()
		if err != nil {
			return
		}
		format := "%s"
		if bytes.IndexAny(b, ` "=`) >= 0 {
			format = "%q"
		}
		fmt.Fprintf(enc.w, format, b)
	default:
		str := fmt.Sprint(v)
		if strings.ContainsAny(str, ` "=`) {
			str = strconv.Quote(str)
		}
		switch str {
		case "nil":
			str = `"nil"`
		case "<nil>":
			str = "nil"
		}
		io.WriteString(enc.w, str)
	}
	return err
}

// EndRecord writes a newline character to the stream and resets the encoder
// to the beginning of a new record.
func (enc *Encoder) EndRecord() error {
	_, err := enc.w.Write(newline)
	if err == nil {
		enc.needSep = false
	}
	return err
}

// Reset resets the encoder to the beginning of a new record.
func (enc *Encoder) Reset() {
	enc.needSep = false
}
