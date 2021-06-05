package datastore

import (
	"bufio"
	"bytes"
	"testing"
)

func TestEntry_Encode(t *testing.T) {
	e := entry{"key", []byte("value")}

	e.Decode(e.Encode())

	if e.key != "key" {
		t.Error("incorrect key")
	}

	if !bytes.Equal(e.value, []byte("value")) {
		t.Error("incorrect value")
	}
}

func TestReadValue(t *testing.T) {
	e := entry{"key", []byte("value")}
	data := e.Encode()

	v, err := readValue(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(v, []byte("value")) {
		t.Errorf("Got bat value [%s]", v)
	}
}
