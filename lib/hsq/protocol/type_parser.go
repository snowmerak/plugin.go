package protocol

import (
	"encoding/binary"
	"errors"
	"io"
	"unsafe"
)

var (
	ErrorInvalidData = errors.New("invalid data")
	ErrorInvalidSize = errors.New("invalid size")
)

func SerializeString(w io.Writer, s string) error {
	buf := [4]byte{}
	length := uint32(len(s))
	binary.BigEndian.PutUint32(buf[0:4], length)
	if _, err := w.Write(buf[0:4]); err != nil {
		return err
	}

	if _, err := w.Write(unsafe.Slice(unsafe.StringData(s), len(s))); err != nil {
		return err
	}

	return nil
}

func DeserializeString(r io.Reader) (string, error) {
	buf := make([]byte, 4)
	n, err := r.Read(buf)
	if err != nil {
		return "", err
	}
	if n < 4 {
		return "", ErrorInvalidSize
	}

	length := binary.BigEndian.Uint32(buf)

	buf = make([]byte, length)
	n, err = r.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	if uint32(n) != length {
		return "", ErrorInvalidSize
	}

	return unsafe.String(unsafe.SliceData(buf), len(buf)), nil
}

func SerializeBytes(w io.Writer, b []byte) error {
	buf := [4]byte{}
	length := uint32(len(b))
	binary.BigEndian.PutUint32(buf[0:4], length)
	if _, err := w.Write(buf[0:4]); err != nil {
		return err
	}

	if _, err := w.Write(b); err != nil {
		return err
	}

	return nil
}

func DeserializeBytes(r io.Reader) ([]byte, error) {
	buf := make([]byte, 4)
	n, err := r.Read(buf)
	if err != nil {
		return nil, err
	}
	if n < 4 {
		return nil, ErrorInvalidSize
	}

	length := binary.BigEndian.Uint32(buf)

	buf = make([]byte, length)
	n, err = r.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	if uint32(n) != length {
		return nil, ErrorInvalidSize
	}

	return buf, nil
}

func SerializeUint64(w io.Writer, v uint64) error {
	buf := [8]byte{}
	binary.BigEndian.PutUint64(buf[0:8], v)
	if _, err := w.Write(buf[0:8]); err != nil {
		return err
	}

	return nil
}

func DeserializeUint64(r io.Reader) (uint64, error) {
	buf := make([]byte, 8)
	n, err := r.Read(buf)
	if err != nil {
		return 0, err
	}
	if n < 8 {
		return 0, ErrorInvalidSize
	}

	return binary.BigEndian.Uint64(buf), nil
}

func SerializeUint32(w io.Writer, v uint32) error {
	buf := [4]byte{}
	binary.BigEndian.PutUint32(buf[0:4], v)
	if _, err := w.Write(buf[0:4]); err != nil {
		return err
	}

	return nil
}

func DeserializeUint32(r io.Reader) (uint32, error) {
	buf := make([]byte, 4)
	n, err := r.Read(buf)
	if err != nil {
		return 0, err
	}
	if n < 4 {
		return 0, ErrorInvalidSize
	}

	return binary.BigEndian.Uint32(buf), nil
}

func SerializeUint16(w io.Writer, v uint16) error {
	buf := [2]byte{}
	binary.BigEndian.PutUint16(buf[0:2], v)
	if _, err := w.Write(buf[0:2]); err != nil {
		return err
	}

	return nil
}

func DeserializeUint16(r io.Reader) (uint16, error) {
	buf := make([]byte, 2)
	n, err := r.Read(buf)
	if err != nil {
		return 0, err
	}
	if n < 2 {
		return 0, ErrorInvalidSize
	}

	return binary.BigEndian.Uint16(buf), nil
}

func SerializeUint8(w io.Writer, v uint8) error {
	buf := [1]byte{}
	buf[0] = v
	if _, err := w.Write(buf[0:1]); err != nil {
		return err
	}

	return nil
}

func DeserializeUint8(r io.Reader) (uint8, error) {
	buf := make([]byte, 1)
	n, err := r.Read(buf)
	if err != nil {
		return 0, err
	}
	if n < 1 {
		return 0, ErrorInvalidSize
	}

	return buf[0], nil
}

func SerializeInt64(w io.Writer, v int64) error {
	buf := [8]byte{}
	binary.BigEndian.PutUint64(buf[0:8], uint64(v))
	if _, err := w.Write(buf[0:8]); err != nil {
		return err
	}

	return nil
}

func DeserializeInt64(r io.Reader) (int64, error) {
	buf := make([]byte, 8)
	n, err := r.Read(buf)
	if err != nil {
		return 0, err
	}
	if n < 8 {
		return 0, ErrorInvalidSize
	}

	return int64(binary.BigEndian.Uint64(buf)), nil
}

func SerializeInt32(w io.Writer, v int32) error {
	buf := [4]byte{}
	binary.BigEndian.PutUint32(buf[0:4], uint32(v))
	if _, err := w.Write(buf[0:4]); err != nil {
		return err
	}

	return nil
}

func DeserializeInt32(r io.Reader) (int32, error) {
	buf := make([]byte, 4)
	n, err := r.Read(buf)
	if err != nil {
		return 0, err
	}
	if n < 4 {
		return 0, ErrorInvalidSize
	}

	return int32(binary.BigEndian.Uint32(buf)), nil
}

func SerializeInt16(w io.Writer, v int16) error {
	buf := [2]byte{}
	binary.BigEndian.PutUint16(buf[0:2], uint16(v))
	if _, err := w.Write(buf[0:2]); err != nil {
		return err
	}

	return nil
}

func DeserializeInt16(r io.Reader) (int16, error) {
	buf := make([]byte, 2)
	n, err := r.Read(buf)
	if err != nil {
		return 0, err
	}
	if n < 2 {
		return 0, ErrorInvalidSize
	}

	return int16(binary.BigEndian.Uint16(buf)), nil
}

func SerializeInt8(w io.Writer, v int8) error {
	buf := [1]byte{}
	buf[0] = byte(v)
	if _, err := w.Write(buf[0:1]); err != nil {
		return err
	}

	return nil
}

func DeserializeInt8(r io.Reader) (int8, error) {
	buf := make([]byte, 1)
	n, err := r.Read(buf)
	if err != nil {
		return 0, err
	}
	if n < 1 {
		return 0, ErrorInvalidSize
	}

	return int8(buf[0]), nil
}

const (
	BoolTrue  = 0b01010101
	BoolFalse = 0b10101010
)

func SerializeBool(w io.Writer, v bool) error {
	buf := [1]byte{}
	if v {
		buf[0] = BoolTrue
	} else {
		buf[0] = BoolFalse
	}
	if _, err := w.Write(buf[0:1]); err != nil {
		return err
	}

	return nil
}

func DeserializeBool(r io.Reader) (bool, error) {
	buf := make([]byte, 1)
	n, err := r.Read(buf)
	if err != nil {
		return false, err
	}
	if n < 1 {
		return false, ErrorInvalidSize
	}

	switch buf[0] {
	case BoolTrue:
		return true, nil
	case BoolFalse:
		return false, nil
	}

	return false, ErrorInvalidData
}
