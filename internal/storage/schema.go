package storage

import (
	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
	"github.com/vmihailenco/msgpack/v5"
)

type StorageKey struct {
	SignalType    string
	AttributeName string
}

func (k StorageKey) String() string {
	return k.SignalType + ":" + k.AttributeName
}

func ParseKey(s string) (signalType, attrName string) {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return s[:i], s[i+1:]
		}
	}
	return "", s
}

func EntryToBytes(entry *dictionary.AttributeEntry) ([]byte, error) {
	return msgpack.Marshal(entry)
}

func BytesToEntry(data []byte) (*dictionary.AttributeEntry, error) {
	entry := &dictionary.AttributeEntry{}
	if err := msgpack.Unmarshal(data, entry); err != nil {
		return nil, err
	}
	return entry, nil
}
