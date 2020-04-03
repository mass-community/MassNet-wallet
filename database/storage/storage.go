package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"massnet.org/mass-wallet/database"
)

const (
	KiB = 1024
	MiB = KiB * 1024
	GiB = MiB * 1024

	// CurrentStorageVersion is the database version.
	CurrentStorageVersion int32 = 1
)

var (
	ErrDbUnknownType      = errors.New("non-existent database type")
	ErrInvalidKey         = errors.New("invalid key")
	ErrInvalidValue       = errors.New("invalid value")
	ErrInvalidBatch       = errors.New("invalid batch")
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrNotFound           = errors.New("not found")
	ErrVersionCheckFailed = errors.New("storage version check failed")
)

// Range is a key range.
type Range struct {
	// Start of the key range, include in the range.
	Start []byte

	// Limit of the key range, not include in the range.
	Limit []byte
}

func (r *Range) IsPrefix() bool {
	pr := BytesPrefix(r.Start)
	expect := pr.Limit
	actual := r.Limit
	switch {
	case len(expect) < len(actual):
		expect = make([]byte, len(actual))
		copy(expect, pr.Limit)
	case len(expect) > len(actual):
		actual = make([]byte, len(expect))
		copy(actual, r.Limit)
	}
	return bytes.Equal(expect, actual)
}

func BytesPrefix(prefix []byte) *Range {
	var limit []byte
	for i := len(prefix) - 1; i >= 0; i-- {
		c := prefix[i]
		if c < 0xff {
			limit = make([]byte, i+1)
			copy(limit, prefix)
			limit[i] = c + 1
			break
		}
	}
	return &Range{Start: prefix, Limit: limit}
}

type Iterator interface {
	Release()
	Error() error
	Seek(key []byte) bool
	Next() bool
	Key() []byte
	Value() []byte
}

type Batch interface {
	Release()
	Put(key, value []byte) error
	Delete(key []byte) error
	Reset()
}

type Storage interface {
	Close() error
	// Get returns ErrNotFound if key not exist
	Get(key []byte) ([]byte, error)
	Put(key, value []byte) error
	Has(key []byte) (bool, error)
	Delete(key []byte) error
	Write(batch Batch) error
	NewBatch() Batch
	NewIterator(slice *Range) Iterator
}

type StorageDriver struct {
	DbType        string
	CreateStorage func(storPath string, args ...interface{}) (s Storage, err error)
	OpenStorage   func(storPath string, args ...interface{}) (s Storage, err error)
}

var drivers []StorageDriver

func RegisterDriver(instance StorageDriver) {
	for _, drv := range drivers {
		if drv.DbType == instance.DbType {
			return
		}
	}
	drivers = append(drivers, instance)
}

// CreateStorage intializes and opens a database.
func CreateStorage(dbtype, dbpath string, args ...interface{}) (s Storage, err error) {
	for _, drv := range drivers {
		if drv.DbType == dbtype {
			s, err = drv.CreateStorage(dbpath, args...)
			if err != nil {
				return nil, err
			}
			// err = CheckVersion(dbtype, dbpath, true)
			// if err != nil {
			// 	return nil, err
			// }
			return
		}
	}
	return nil, ErrDbUnknownType
}

// CreateStorage opens an existing database.
func OpenStorage(dbtype, dbpath string, args ...interface{}) (s Storage, err error) {
	// err = CheckVersion(dbtype, dbpath, false)
	// if err != nil {
	// 	return nil, err
	// }
	for _, drv := range drivers {
		if drv.DbType == dbtype {
			return drv.OpenStorage(dbpath, args...)
		}
	}
	return nil, ErrDbUnknownType
}

func RegisteredDbTypes() []string {
	var types []string
	for _, drv := range drivers {
		types = append(types, drv.DbType)
	}
	return types
}

type storageVersion struct {
	Dbtype  string `json:"dbtype,omitempty"`
	Version int32  `json:"version,omitempty"`
}

func CheckVersion(dbtype, storPath string, create bool) error {
	verFile := filepath.Join(storPath, ".ver")
	fmt.Println("ver file path:", verFile, storPath, create)
	if create {
		fo, err := os.Create(verFile)
		if err != nil {
			return fmt.Errorf("create ver file error: %v", err)
		}
		defer fo.Close()

		b, err := json.Marshal(storageVersion{
			Dbtype:  dbtype,
			Version: CurrentStorageVersion,
		})
		if err != nil {
			return fmt.Errorf("marshal failed: %v", err)
		}
		return binary.Write(fo, binary.LittleEndian, b)
	}

	// open
	fi, err := os.Open(verFile)
	if os.IsNotExist(err) {
		return database.ErrDbDoesNotExist
	}
	if err == nil {
		defer fi.Close()

		fs, err := fi.Stat()
		if err != nil {
			return err
		}
		buf := make([]byte, fs.Size())
		err = binary.Read(fi, binary.LittleEndian, buf)
		if err != nil {
			return fmt.Errorf("read version file error: %v", err)
		}

		var ver storageVersion
		err = json.Unmarshal(buf, &ver)
		if err != nil {
			return fmt.Errorf("unmarshal failed: %v", err)
		}

		if ver.Version == CurrentStorageVersion && ver.Dbtype == dbtype {
			return nil
		}
	}
	return ErrVersionCheckFailed
}
