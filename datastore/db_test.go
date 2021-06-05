package datastore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	dataset = [][]string{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
	}

	anotherDataset = [][]string{
		{"key2", "value1"},
		{"key3", "value2"},
	}

	bigDataset = [][]string{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
		{"key4", "value4"},
		{"key5", "value5"},
		{"key6", "value6"},
		{"key7", "value7"},
		{"key8", "value8"},
		{"key9", "value9"},
		{"key10", "value10"},
		{"key11", "value11"},
		{"key12", "value12"},
	}
)

func TestDatastore_Put(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := NewDatastore(dir)
	if err != nil {
		t.Fatal(err)
	}

	output, err := os.Open(filepath.Join(dir, segmentPrefix+currentSegmentSuffix))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("put/get", func(t *testing.T) {
		for _, pair := range dataset {
			err := db.Put(pair[0], pair[1])
			if err != nil {
				t.Errorf("can't put %s: %s", dataset[0], err)
			}
			value, err := db.Get(pair[0])
			if err != nil {
				t.Errorf("can't get %s: %s", dataset[0], err)
			}
			if value != pair[1] {
				t.Errorf("wrong value returned expected %s, got %s", pair[1], value)
			}
		}
	})

	outInfo, err := output.Stat()
	if err != nil {
		t.Fatal(err)
	}
	size1 := outInfo.Size()

	t.Run("incremental write", func(t *testing.T) {
		for _, pair := range dataset {
			err := db.Put(pair[0], pair[1])
			if err != nil {
				t.Errorf("can't put %s: %s", dataset[0], err)
			}
		}
		outInfo, err := output.Stat()
		if err != nil {
			t.Fatal(err)
		}
		if size1*2 != outInfo.Size() {
			t.Errorf("unexpected size, got %d instead of %d", outInfo.Size(), size1*2)
		}
	})

	t.Run("new db process", func(t *testing.T) {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		db, err = NewDatastore(dir)
		if err != nil {
			t.Fatal(err)
		}

		for _, pair := range dataset {
			value, err := db.Get(pair[0])
			if err != nil {
				t.Errorf("can't get %s: %s", dataset[0], err)
			}
			if value != pair[1] {
				t.Errorf("wrong value returned expected %s, got %s", pair[1], value)
			}
		}
	})
}
func TestDatastore_Segmentation(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := NewDatastoreOfSize(dir, 50)
	if err != nil {
		t.Fatal(err)
	}

	for _, pair := range dataset {
		err = db.Put(pair[0], pair[1])
		if err != nil {
			t.Fatal(err)
		}
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Error(err)
	}
	if len(files) != 2 {
		t.Errorf("unexpected segment count, got %d instead of %d", len(files), 2)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDatastore_Merge(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := NewDatastoreMergeToSize(dir, 44, false)
	if err != nil {
		t.Fatal(err)
	}

	for _, pair := range dataset {
		err = db.Put(pair[0], pair[1])
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, pair := range anotherDataset {
		err = db.Put(pair[0], pair[1])
		if err != nil {
			t.Fatal(err)
		}
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Error(err)
	}
	if len(files) != 3 {
		t.Errorf("unexpected segment count before merge, got %d instead of %d", len(files), 3)
	}

	db.merge()
	files, err = ioutil.ReadDir(dir)
	if err != nil {
		t.Error(err)
	}
	if len(files) != 2 {
		t.Errorf("unexpected segment count after merge, got %d instead of %d", len(files), 2)
	}

	mergedSegment := db.segments[1]
	expectedMergedSegment := [][]string{
		{"key1", "value1"},
		{"key2", "value1"},
		{"key3", "value3"},
	}

	for _, pair := range expectedMergedSegment {
		value, err := mergedSegment.get(pair[0])
		if err != nil {
			t.Errorf("can't get %s: %s", pair[0], err)
		}

		if value != pair[1] {
			t.Errorf("wrong value returned expected %s, got %s", pair[1], value)
		}
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDatastore_Concurrency(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := NewDatastoreOfSize(dir, 44)
	if err != nil {
		t.Fatal(err)
	}

	resultChannel := make(chan int)

	for _, pair := range bigDataset {
		pair := pair
		go func() {
			err := db.Put(pair[0], pair[1])
			if err != nil {
				t.Errorf("can't put %s: %s", pair[0], err)
			}

			value, err := db.Get(pair[0])
			if err != nil {
				t.Errorf("can't get %s: %s", pair[0], err)
			}

			if value != pair[1] {
				t.Errorf("wrong value returned expected %s, got %s", pair[1], value)
			}

			resultChannel <- 1
		}()
	}

	for range bigDataset {
		<-resultChannel
	}

	for _, pair := range bigDataset {
		value, err := db.Get(pair[0])
		if err != nil {
			t.Errorf("can't get %s: %s", pair[0], err)
		}

		if value != pair[1] {
			t.Errorf("wrong value returned expected %s, got %s", pair[1], value)
		}
	}

	time.Sleep(1 * time.Second)

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}
