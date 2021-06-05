package datastore

import (
	"bufio"
	"bytes"
	"testing"
)

func TestEntry_Encode(t *testing.T) {
	e := entry{"key", "value"}
	e.Decode(e.Encode())
	if e.key != "key" {
		t.Error("wrong key")
	}
	if e.value != "value" {
		t.Error("wrong value")
	}
}

func TestReadValue(t *testing.T) {
	e := entry{"key", "value"}
	data := e.Encode()
	v, err := readValue(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		t.Fatal(err)
	}
	if v != "value" {
		t.Errorf("wrong value [%s]", v)
	}
}
