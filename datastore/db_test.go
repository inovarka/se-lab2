package datastore

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	dataset = map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	anotherDataset = map[string][]byte{
		"key2": []byte("value1"),
		"key3": []byte("value2"),
	}

	bigDataset = map[string][]byte{
		"key1":  []byte("value1"),
		"key2":  []byte("value2"),
		"key3":  []byte("value3"),
		"key4":  []byte("value4"),
		"key5":  []byte("value5"),
		"key6":  []byte("value6"),
		"key7":  []byte("value7"),
		"key8":  []byte("value8"),
		"key9":  []byte("value9"),
		"key10": []byte("value10"),
		"key11": []byte("value11"),
		"key12": []byte("value12"),
	}
)

func TestDatastore_Put(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}

	defer func(path string) {
		err = os.RemoveAll(path)
		if err != nil {
			t.Log(err)
		}
	}(dir)

	db, err := NewDatastore(dir)
	if err != nil {
		t.Fatal(err)
	}

	output, err := os.Open(filepath.Join(dir, segmentPrefix+currentSegmentSuffix))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("put/get", func(t *testing.T) {
		for key, val := range dataset {
			if err = db.Put(key, val); err != nil {
				t.Errorf("can't put %s: %s", key, err)
			}

			var value []byte

			if value, err = db.Get(key); err != nil {
				t.Errorf("can't get %s: %s", key, err)
			}

			if !bytes.Equal(value, val) {
				t.Errorf("wrong value returned expected %s, got %s", val, value)
			}
		}
	})

	outInfo, err := output.Stat()
	if err != nil {
		t.Fatal(err)
	}

	size1 := outInfo.Size()

	t.Run("incremental write", func(t *testing.T) {
		for key, val := range dataset {
			if err = db.Put(key, val); err != nil {
				t.Errorf("can't put %s: %s", key, err)
			}
		}

		if outInfo, err = output.Stat(); err != nil {
			t.Fatal(err)
		}

		if size1*2 != outInfo.Size() {
			t.Errorf("unexpected size, got %d instead of %d", outInfo.Size(), size1*2)
		}
	})

	t.Run("new db process", func(t *testing.T) {
		if err = db.Close(); err != nil {
			t.Fatal(err)
		}

		if db, err = NewDatastore(dir); err != nil {
			t.Fatal(err)
		}

		for key, val := range dataset {
			value, err := db.Get(key)
			if err != nil {
				t.Errorf("can't get %s: %s", key, err)
			}

			if !bytes.Equal(value, val) {
				t.Errorf("wrong value returned expected %s, got %s", val, value)
			}
		}
	})
}

func TestDatastore_Segmentation(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}

	defer func(path string) {
		err = os.RemoveAll(path)
		if err != nil {
			t.Log(err)
		}
	}(dir)

	db, err := NewDatastoreOfSize(dir, 50)
	if err != nil {
		t.Fatal(err)
	}

	for key, val := range dataset {
		if err = db.Put(key, val); err != nil {
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

	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDatastore_Merge(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}

	defer func(path string) {
		err = os.RemoveAll(path)
		if err != nil {
			t.Log(err)
		}
	}(dir)

	db, err := NewDatastoreMergeToSize(dir, 44, false)
	if err != nil {
		t.Fatal(err)
	}

	for key, val := range dataset {
		if err = db.Put(key, val); err != nil {
			t.Fatal(err)
		}
	}

	for key, val := range anotherDataset {
		if err = db.Put(key, val); err != nil {
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

	_ = db.merge()

	files, err = ioutil.ReadDir(dir)
	if err != nil {
		t.Error(err)
	}

	if len(files) != 2 {
		t.Errorf("unexpected segment count after merge, got %d instead of %d", len(files), 2)
	}

	mergedSegment := db.segments[1]
	expectedMergedSegment := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value1"),
		"key3": []byte("value3"),
	}

	for key, val := range expectedMergedSegment {
		value, err := mergedSegment.get(key)
		if err != nil {
			t.Errorf("can't get %s: %s", key, err)
		}

		if !bytes.Equal(value, val) {
			t.Errorf("wrong value returned expected %s, got %s", val, value)
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

	defer func(path string) {
		err = os.RemoveAll(path)
		if err != nil {
			t.Log(err)
		}
	}(dir)

	db, err := NewDatastoreOfSize(dir, 44)
	if err != nil {
		t.Fatal(err)
	}

	resultChannel := make(chan int)

	for key, val := range bigDataset {
		key, val := key, val

		go func() {
			if err = db.Put(key, val); err != nil {
				t.Errorf("can't put %s: %s", key, err)
			}

			var value []byte

			if value, err = db.Get(key); err != nil {
				t.Errorf("can't get %s: %s", key, err)
			}

			if !bytes.Equal(value, val) {
				t.Errorf("wrong value returned expected %s, got %s", val, value)
			}

			resultChannel <- 1
		}()
	}

	for range bigDataset {
		<-resultChannel
	}

	for key, val := range bigDataset {
		value, err := db.Get(key)
		if err != nil {
			t.Errorf("can't get %s: %s", key, err)
		}

		if !bytes.Equal(value, val) {
			t.Errorf("wrong value returned expected %s, got %s", val, value)
		}
	}

	time.Sleep(1 * time.Second)

	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
}
