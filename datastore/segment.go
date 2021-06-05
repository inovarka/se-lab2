package datastore

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const currentSegmentSuffix = ".current"
const mergedSegmentSuffix = ".merged"
const segmentPrefix = "segment."
const bufferSize = 8192

type segment struct {
	path   string
	offset int64
	index  hashIndex
}

func (s *segment) restore() error {
	input, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer input.Close()

	var buffer [bufferSize]byte
	in := bufio.NewReaderSize(input, bufferSize)

	for err == nil {
		var (
			header, data []byte
			n            int
		)
		header, err = in.Peek(bufferSize)
		if err == io.EOF {
			if len(header) == 0 {
				return err
			}
		} else if err != nil {
			return err
		}
		size := binary.LittleEndian.Uint32(header)

		if size < bufferSize {
			data = buffer[:size]
		} else {
			data = make([]byte, size)
		}
		n, err = in.Read(data)

		if err == nil {
			if n != int(size) {
				return fmt.Errorf("corrupted file")
			}

			var e entry
			e.Decode(data)
			s.index[e.key] = s.offset

			s.offset += int64(n)
		}
	}
	return err
}

func (s *segment) get(key string) (string, error) {

	position, ok := s.index[key]

	if !ok && position != 0 {
		print()
	}
	if !ok {
		return "", fmt.Errorf("entry does not exist")
	}

	file, err := os.Open(s.path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = file.Seek(position, 0)
	if err != nil {
		return "", err
	}

	reader := bufio.NewReader(file)
	value, err := readValue(reader)
	if err != nil {
		return "", err
	}

	return value, nil
}
