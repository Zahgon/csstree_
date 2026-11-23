package ast

// Item is one place in a List. An item keeps its identity while the list around it changes, so a
// walk that removes or inserts nodes can carry on from the item it is on.
type Item struct {
	Data *Node
	Prev *Item
	Next *Item
}

// List is the doubly linked list of child nodes the tree holds. The parser builds one per node
// that has children, and a walk hands the callback the item a child sits in so that the callback
// can change the list around it.
type List struct {
	Head *Item
	Tail *Item
	size int
}

// NewList returns an empty list.
func NewList() *List {
	return &List{}
}

// FromSlice fills an empty list with the nodes, in order.
func (l *List) FromSlice(nodes []*Node) *List {
	for _, node := range nodes {
		l.AppendData(node)
	}
	return l
}

// Len is the number of items.
func (l *List) Len() int {
	if l == nil {
		return 0
	}
	return l.size
}

// IsEmpty reports whether the list holds nothing.
func (l *List) IsEmpty() bool {
	return l == nil || l.Head == nil
}

// First is the first node, or nil.
func (l *List) First() *Node {
	if l == nil || l.Head == nil {
		return nil
	}
	return l.Head.Data
}

// Last is the last node, or nil.
func (l *List) Last() *Node {
	if l == nil || l.Tail == nil {
		return nil
	}
	return l.Tail.Data
}

// AppendData adds a node at the end and returns its item.
func (l *List) AppendData(node *Node) *Item {
	item := &Item{Data: node}
	return l.Append(item)
}

// Append adds an item at the end.
func (l *List) Append(item *Item) *Item {
	return l.InsertBefore(item, nil)
}

// PrependData adds a node at the front and returns its item.
func (l *List) PrependData(node *Node) *Item {
	item := &Item{Data: node}
	return l.InsertBefore(item, l.Head)
}

// InsertBefore puts an item in front of before, or at the end when before is nil.
func (l *List) InsertBefore(item *Item, before *Item) *Item {
	if before == nil {
		if l.Tail == nil {
			l.Head = item
			l.Tail = item
			item.Prev = nil
			item.Next = nil
		} else {
			item.Prev = l.Tail
			item.Next = nil
			l.Tail.Next = item
			l.Tail = item
		}
	} else {
		item.Prev = before.Prev
		item.Next = before
		if before.Prev == nil {
			l.Head = item
		} else {
			before.Prev.Next = item
		}
		before.Prev = item
	}
	l.size++
	return item
}

// Remove takes an item out of the list and returns it.
func (l *List) Remove(item *Item) *Item {
	if item.Prev != nil {
		item.Prev.Next = item.Next
	} else {
		l.Head = item.Next
	}
	if item.Next != nil {
		item.Next.Prev = item.Prev
	} else {
		l.Tail = item.Prev
	}
	item.Prev = nil
	item.Next = nil
	l.size--
	return item
}

// Slice copies the nodes into a slice, in order.
func (l *List) Slice() []*Node {
	if l == nil {
		return nil
	}
	out := make([]*Node, 0, l.size)
	for item := l.Head; item != nil; item = item.Next {
		out = append(out, item.Data)
	}
	return out
}

// Each calls fn for every item from the front, reading each item's successor before the call so
// that fn may remove the item it is given.
func (l *List) Each(fn func(node *Node, item *Item, list *List)) {
	if l == nil {
		return
	}
	for item := l.Head; item != nil; {
		next := item.Next
		fn(item.Data, item, l)
		item = next
	}
}

// EachRight calls fn for every item from the back, in the same way.
func (l *List) EachRight(fn func(node *Node, item *Item, list *List)) {
	if l == nil {
		return
	}
	for item := l.Tail; item != nil; {
		prev := item.Prev
		fn(item.Data, item, l)
		item = prev
	}
}
