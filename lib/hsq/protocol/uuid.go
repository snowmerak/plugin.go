package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"strings"

	"github.com/google/uuid"
)

func GenerateNumberID() (uint32, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(buf), nil
}

func GenerateUUID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}

	s := id.String()
	s = strings.Replace(s, "-", "", -1)
	return s, nil
}
