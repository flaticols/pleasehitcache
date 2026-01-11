package slice

import "sync"

// Should warn against padding because it's used as slice element
type SliceItem struct { // want `padding NOT recommended`
	mu    sync.Mutex
	value int64
}

// This usage makes SliceItem a slice element
var items []SliceItem

func addItem(item SliceItem) {
	items = append(items, item)
}
