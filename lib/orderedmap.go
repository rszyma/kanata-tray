package lib

import (
	"fmt"
	"strings"

	"github.com/elliotchance/orderedmap/v2"
)

type OrderedMap[K string, V fmt.GoStringer] struct {
	*orderedmap.OrderedMap[K, V]
}

func NewOrderedMap[K string, V fmt.GoStringer]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{
		OrderedMap: orderedmap.NewOrderedMap[K, V](),
	}
}

type Entry[K comparable, V any] struct {
	Key   K
	Value V
}

func NewOrderedMapFromIter[K string, V fmt.GoStringer](iter []Entry[K, V]) *OrderedMap[K, V] {
	m := &OrderedMap[K, V]{
		OrderedMap: orderedmap.NewOrderedMap[K, V](),
	}
	for _, elem := range iter {
		m.Set(elem.Key, elem.Value)
	}
	return m
}

// impl `fmt.GoStringer`
func (m *OrderedMap[K, V]) GoString() string {
	indent := "    "
	keys := []K{}
	values := []V{}
	for it := m.Front(); it != nil; it = it.Next() {
		keys = append(keys, it.Key)
		values = append(values, it.Value)
	}
	builder := strings.Builder{}
	builder.WriteString("{")
	for i := range keys {
		key := keys[i]
		value := values[i]
		valueLines := strings.Split(value.GoString(), "\n")
		for i, vl := range valueLines {
			if i == 0 {
				continue
			}
			valueLines[i] = fmt.Sprintf("%s%s", indent, vl)
		}
		indentedVal := strings.Join(valueLines, "\n")
		builder.WriteString(fmt.Sprintf("\n%s\"%s\": %s", indent, key, indentedVal))
	}
	builder.WriteString("\n}")
	return builder.String()
}

func (m *OrderedMap[K, V]) Entries() []Entry[K, V] {
	if m == nil {
		return nil
	}
	entries := make([]Entry[K, V], m.Len())
	for it := m.Front(); it != nil; it = it.Next() {
		entries = append(entries, Entry[K, V]{
			Key:   it.Key,
			Value: it.Value,
		})
	}
	return entries
}
