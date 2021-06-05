package datastore

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
)

const (
	currentSegmentSuffix = ".current"
	mergedSegmentSuffix  = ".merged"
	segmentPrefix        = "segment."
	bufferSize           = 8192
)

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

	defer func(input *os.File) {
		err = input.Close()
		if err != nil {
			log.Panic(err)
		}
	}(input)

	var buffer [bufferSize]byte

	in := bufio.NewReaderSize(input, bufferSize)

	for err == nil {
		var (
			header, data []byte
			n            int
		)

		header, err = in.Peek(bufferSize)
		if errors.Is(err, io.EOF) {
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
				return ErrCorruptedFile
			}

			var e entry

			e.Decode(data)

			s.index[e.key] = s.offset
			s.offset += int64(n)
		}
	}

	return err
}

func (s *segment) get(key string) ([]byte, error) {
	position, ok := s.index[key]
	if !ok {
		if position != 0 {
			print()
		} else {
			return nil, ErrNotFound
		}
	}

	file, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}

	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			log.Panic(err)
		}
	}(file)

	if _, err = file.Seek(position, 0); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(file)

	value, err := readValue(reader)
	if err != nil {
		return nil, err
	}

	return value, nil
}
