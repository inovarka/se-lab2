package datastore

import (
	"bufio"
	"encoding/binary"
	"fmt"
)

type entry struct {
	key   string
	value []byte
}

func (e *entry) Encode() []byte {
	kl := len(e.key)
	vl := len(e.value)
	size := kl + vl + 12
	res := make([]byte, size)

	binary.LittleEndian.PutUint32(res, uint32(size))
	binary.LittleEndian.PutUint32(res[4:], uint32(kl))
	copy(res[8:], e.key)
	binary.LittleEndian.PutUint32(res[kl+8:], uint32(vl))
	copy(res[kl+12:], e.value)

	return res
}

func (e *entry) Decode(input []byte) {
	kl := binary.LittleEndian.Uint32(input[4:])
	keyBuf := make([]byte, kl)

	copy(keyBuf, input[8:kl+8])

	e.key = string(keyBuf)

	vl := binary.LittleEndian.Uint32(input[kl+8:])
	valBuf := make([]byte, vl)

	copy(valBuf, input[kl+12:kl+12+vl])

	e.value = valBuf
}

func readValue(in *bufio.Reader) ([]byte, error) {
	header, err := in.Peek(8)
	if err != nil {
		return nil, err
	}

	keySize := int(binary.LittleEndian.Uint32(header[4:]))

	if _, err = in.Discard(keySize + 8); err != nil {
		return nil, err
	}

	header, err = in.Peek(4)
	if err != nil {
		return nil, err
	}

	valSize := int(binary.LittleEndian.Uint32(header))

	if _, err = in.Discard(4); err != nil {
		return nil, err
	}

	data := make([]byte, valSize)

	n, err := in.Read(data)
	if err != nil {
		return nil, err
	}

	if n != valSize {
		return nil, fmt.Errorf("can't read value bytes (read %d, expected %d)", n, valSize)
	}

	return data, nil
}
