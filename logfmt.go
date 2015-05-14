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
	nilbytes = []byte("null")
)

// ErrNilKey is returned by Marshal functions and Encoder methods if a key is
// a nil interface or pointer value.
var ErrNilKey = errors.New("nil key")

// ErrInvalidKey is returned by Marshal functions and Encoder methods if a key
// contains an invalid character.
var ErrInvalidKey = errors.New("invalid key")

// ErrUnsportedType is returned by Marshal functions and Encoder methods if a
// key or value has an unsupported type.
var ErrUnsportedType = errors.New("unsupported type")

// EncodeKeyval writes the logfmt encoding of key and value to the stream. A
// single space is written before the second and subsequent keys in a record.
func (enc *Encoder) EncodeKeyval(key, value interface{}) error {
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

func (enc *Encoder) writeKey(key interface{}) error {
	if key == nil {
		return ErrNilKey
	}

	switch k := key.(type) {
	case string:
		return enc.writeStringKey(k)
	case encoding.TextMarshaler:
		kb, err := safeMarshal(k)
		if err != nil {
			return err
		}
		if kb == nil {
			return ErrNilKey
		}
		return enc.writeBytesKey(kb)
	case fmt.Stringer:
		ks, ok := safeString(k)
		if !ok {
			return ErrNilKey
		}
		return enc.writeStringKey(ks)
	default:
		rkey := reflect.ValueOf(key)
		switch rkey.Kind() {
		case reflect.Array, reflect.Chan, reflect.Func, reflect.Map, reflect.Slice, reflect.Struct:
			return ErrUnsportedType
		case reflect.Ptr:
			if rkey.IsNil() {
				return ErrNilKey
			}
		}
		return enc.writeStringKey(fmt.Sprint(k))
	}
}

func invalidKeyRune(r rune) bool {
	return r <= ' ' || r == '=' || r == '"'
}

func (enc *Encoder) writeStringKey(key string) error {
	if len(key) == 0 || strings.IndexFunc(key, invalidKeyRune) != -1 {
		return ErrInvalidKey
	}
	_, err := io.WriteString(enc.w, key)
	return err
}

func (enc *Encoder) writeBytesKey(key []byte) error {
	if len(key) == 0 || bytes.IndexFunc(key, invalidKeyRune) != -1 {
		return ErrInvalidKey
	}
	_, err := enc.w.Write(key)
	return err
}

func (enc *Encoder) writeValue(value interface{}) error {
	switch v := value.(type) {
	case nil:
		return enc.writeBytesValue(nilbytes)
	case string:
		return enc.writeStringValue(v, true)
	case encoding.TextMarshaler:
		vb, err := safeMarshal(v)
		if err != nil {
			return err
		}
		if vb == nil {
			vb = nilbytes
		}
		return enc.writeBytesValue(vb)
	case fmt.Stringer:
		return enc.writeStringValue(safeString(v))
	default:
		rvalue := reflect.ValueOf(value)
		switch rvalue.Kind() {
		case reflect.Array, reflect.Chan, reflect.Func, reflect.Map, reflect.Slice, reflect.Struct:
			return ErrUnsportedType
		case reflect.Ptr:
			if rvalue.IsNil() {
				return enc.writeBytesValue(nilbytes)
			}
		}
		vs := fmt.Sprint(v)
		return enc.writeStringValue(vs, true)
	}
}

func needsQuotedValueRune(r rune) bool {
	return r <= ' ' || r == '=' || r == '"'
}

func (enc *Encoder) writeStringValue(value string, ok bool) error {
	var err error
	if ok && value == "null" {
		_, err = io.WriteString(enc.w, `"null"`)
	} else if strings.IndexFunc(value, needsQuotedValueRune) != -1 {
		_, err = enc.writeQuotedString(value)
	} else {
		_, err = io.WriteString(enc.w, value)
	}
	return err
}

func (enc *Encoder) writeBytesValue(value []byte) error {
	var err error
	if bytes.IndexFunc(value, needsQuotedValueRune) >= 0 {
		_, err = enc.writeQuotedBytes(value)
	} else {
		_, err = enc.w.Write(value)
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

func safeString(str fmt.Stringer) (s string, ok bool) {
	defer func() {
		if panicVal := recover(); panicVal != nil {
			if v := reflect.ValueOf(str); v.Kind() == reflect.Ptr && v.IsNil() {
				s, ok = "null", false
			} else {
				panic(panicVal)
			}
		}
	}()
	s, ok = str.String(), true
	return
}

func safeMarshal(tm encoding.TextMarshaler) (b []byte, err error) {
	defer func() {
		if panicVal := recover(); panicVal != nil {
			if v := reflect.ValueOf(tm); v.Kind() == reflect.Ptr && v.IsNil() {
				b, err = nil, nil
			} else {
				panic(panicVal)
			}
		}
	}()
	b, err = tm.MarshalText()
	return
}
