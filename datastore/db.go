package datastore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/semaphore"
)

const (
	maxReadThreads = 8
	maxBlockSize   = 10 * 1024 * 1024
)

var (
	ErrNotFound      = errors.New("entry does not exist")
	ErrCorruptedFile = errors.New("corrupted file")
)

type hashIndex map[string]int64

type putQuery struct {
	entry    *entry
	callback chan error
}

type Datastore struct {
	mutex     *sync.RWMutex
	semaphore *semaphore.Weighted
	out       *os.File

	dir              string
	currentBlockSize int64
	mergingPolicy    bool

	segments       []*segment
	mergingChannel chan int
	putChannel     chan putQuery
}

func NewDatastore(dir string) (*Datastore, error) {
	return NewDatastoreMergeToSize(dir, maxBlockSize, true)
}

func NewDatastoreMerge(dir string, mergingPolicy bool) (*Datastore, error) {
	return NewDatastoreMergeToSize(dir, maxBlockSize, mergingPolicy)
}

func NewDatastoreOfSize(dir string, currentBlockSize int64) (*Datastore, error) {
	return NewDatastoreMergeToSize(dir, currentBlockSize, true)
}

func NewDatastoreMergeToSize(dir string, currentBlockSize int64, mergingPolicy bool) (*Datastore, error) {
	outputPath := filepath.Join(dir, segmentPrefix+currentSegmentSuffix)

	f, err := os.OpenFile(outputPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}

	var segments []*segment

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, fileInfo := range files {
		if strings.HasPrefix(fileInfo.Name(), segmentPrefix) {
			s := &segment{
				path:  filepath.Join(dir, fileInfo.Name()),
				index: make(hashIndex),
			}

			if err = s.restore(); !errors.Is(err, io.EOF) {
				return nil, err
			}

			segments = append(segments, s)
		}
	}

	sort.Slice(segments, func(n, m int) bool {
		suffixFirst := segments[n].path[len(dir+segmentPrefix)+1:]
		suffixSecond := segments[m].path[len(dir+segmentPrefix)+1:]

		if suffixFirst == currentSegmentSuffix || suffixSecond == mergedSegmentSuffix {
			return true
		}

		if suffixSecond == currentSegmentSuffix || suffixFirst == mergedSegmentSuffix {
			return false
		}

		suffixN, errN := strconv.Atoi(suffixFirst)
		suffixM, errM := strconv.Atoi(suffixSecond)

		return errM != nil || (errN != nil && suffixN > suffixM)
	})

	mergingChannel := make(chan int)
	putChannel := make(chan putQuery)

	db := &Datastore{
		mutex:            new(sync.RWMutex),
		semaphore:        semaphore.NewWeighted(maxReadThreads),
		out:              f,
		dir:              dir,
		currentBlockSize: currentBlockSize,
		mergingPolicy:    mergingPolicy,
		segments:         segments,
		mergingChannel:   mergingChannel,
		putChannel:       putChannel,
	}

	go func() {
		for el := range mergingChannel {
			if el == 0 {
				return
			}

			_ = db.merge()
		}
	}()

	go func() {
		for el := range putChannel {
			if el.entry == nil {
				return
			}

			_ = db.put(el)
		}
	}()

	return db, nil
}

func (db *Datastore) Close() error {
	db.mergingChannel <- 0
	db.putChannel <- putQuery{entry: nil}

	return db.out.Close()
}

func (db *Datastore) Get(key string) ([]byte, error) {
	if err := db.semaphore.Acquire(context.TODO(), 1); err != nil {
		return nil, err
	}

	defer db.semaphore.Release(1)

	var (
		value []byte
		err   error
	)

	for _, seg := range db.segments {
		if value, err = seg.get(key); err == nil {
			return value, nil
		}
	}

	return nil, err
}

func (db *Datastore) Put(key string, value []byte) error {
	callback := make(chan error)
	e := &entry{key: key, value: value}

	db.putChannel <- putQuery{entry: e, callback: callback}

	res := <-callback

	return res
}

func (db *Datastore) put(pe putQuery) error {
	if len(db.segments) > 2 && db.mergingPolicy {
		go func() {
			db.mergingChannel <- 1
		}()
	}

	e := pe.entry

	n, err := db.out.Write(e.Encode())
	if err != nil {
		pe.callback <- err

		return err
	}

	db.mutex.Lock()

	activeSegment := db.segments[0]
	activeSegment.index[e.key] = activeSegment.offset
	activeSegment.offset += int64(n)

	db.mutex.Unlock()

	fi, err := os.Stat(activeSegment.path)
	if err != nil {
		pe.callback <- nil

		return fmt.Errorf("can not read active file stat: %v", err)
	}

	if fi.Size() >= db.currentBlockSize {
		_, err = db.addSegment()
		if err != nil {
			pe.callback <- nil

			return err
		}
	}

	pe.callback <- nil

	return nil
}

func (db *Datastore) addSegment() (*segment, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err := db.out.Close(); err != nil {
		return nil, err
	}

	segmentSuffix := 0

	if len(db.segments) > 1 {
		var (
			lastSavedSegmentSuffix = db.segments[1].path[len(db.dir+segmentPrefix)+1:]
		)

		if prevSegmentSuffix, err := strconv.Atoi(lastSavedSegmentSuffix); err == nil {
			segmentSuffix = prevSegmentSuffix + 1
		}
	}

	segmentPath := filepath.Join(db.dir, fmt.Sprintf("%v%v", segmentPrefix, segmentSuffix))
	outputPath := filepath.Join(db.dir, segmentPrefix+currentSegmentSuffix)

	if err := os.Rename(outputPath, segmentPath); err != nil {
		return nil, err
	}

	db.segments[0].path = segmentPath

	f, err := os.OpenFile(outputPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}

	db.out = f

	s := &segment{
		path:  outputPath,
		index: make(hashIndex),
	}
	db.segments = append([]*segment{s}, db.segments...)

	return s, nil
}

func (db *Datastore) merge() error {
	toMerge := db.segments[1:]
	segments := make([]*segment, len(toMerge))

	copy(segments, toMerge)

	if len(segments) < 2 {
		return fmt.Errorf("not enough segments to merge")
	}

	keysSegments := make(map[string]*segment)

	for i := len(segments) - 1; i >= 0; i-- {
		s := segments[i]
		for k := range segments[i].index {
			keysSegments[k] = s
		}
	}

	segmentPath := filepath.Join(db.dir, segmentPrefix)

	f, err := os.OpenFile(segmentPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("error occured during merging: %v", err)
	}

	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			log.Panic(err)
		}
	}(f)

	seg := &segment{
		path:  segmentPath,
		index: make(hashIndex),
	}

	for k, s := range keysSegments {
		var value []byte

		if value, err = s.get(k); value != nil && err == nil {
			e := (&entry{
				key:   k,
				value: value,
			}).Encode()

			n, err := f.Write(e)
			if err != nil {
				return fmt.Errorf("error occured during merging: %v", err)
			}

			seg.index[k] = seg.offset
			seg.offset += int64(n)
		}
	}

	db.mutex.Lock()

	newPath := segmentPath + mergedSegmentSuffix

	if err = os.Rename(segmentPath, newPath); err != nil {
		db.mutex.Unlock()

		return fmt.Errorf("can't merge: %v", err)
	}

	seg.path = newPath
	to := len(db.segments) - len(segments)
	db.segments = append(db.segments[:to], seg)

	db.mutex.Unlock()

	for _, s := range segments {
		if newPath != s.path {
			err = os.Remove(s.path)
			if err != nil {
				log.Panic(err)
			}
		}
	}

	return nil
}
