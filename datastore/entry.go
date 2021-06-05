package datastore

import (
	"bufio"
	"encoding/binary"
	"fmt"
)

type entry struct {
	key, value string
}

func (e *entry) Encode() []byte {
	keySize := len(e.key)
	valueSize := len(e.value)
	size := keySize + valueSize + 12
	res := make([]byte, size)
	binary.LittleEndian.PutUint32(res, uint32(size))
	binary.LittleEndian.PutUint32(res[4:], uint32(keySize))
	copy(res[8:], e.key)
	binary.LittleEndian.PutUint32(res[keySize+8:], uint32(valueSize))
	copy(res[keySize+12:], e.value)
	return res
}

func (e *entry) Decode(input []byte) {
	keySize := binary.LittleEndian.Uint32(input[4:])
	keyBuffer := make([]byte, keySize)
	copy(keyBuffer, input[8:keySize+8])
	e.key = string(keyBuffer)

	valueSize := binary.LittleEndian.Uint32(input[keySize+8:])
	valueBuffer := make([]byte, valueSize)
	copy(valueBuffer, input[keySize+12:keySize+12+valueSize])
	e.value = string(valueBuffer)
}

func readValue(in *bufio.Reader) (string, error) {
	header, err := in.Peek(8)
	if err != nil {
		return "", err
	}
	keySize := int(binary.LittleEndian.Uint32(header[4:]))
	_, err = in.Discard(keySize + 8)
	if err != nil {
		return "", err
	}

	header, err = in.Peek(4)
	if err != nil {
		return "", err
	}
	valueSize := int(binary.LittleEndian.Uint32(header))
	_, err = in.Discard(4)
	if err != nil {
		return "", err
	}

	data := make([]byte, valueSize)
	n, err := in.Read(data)
	if err != nil {
		return "", err
	}
	if n != valueSize {
		return "", fmt.Errorf("can't read value bytes (read %d, expected %d)", n, valueSize)
	}

	return string(data), nil
}
