# The List type

A node that has children holds them in a `csstree.List`, a doubly linked list. The list is linked
rather than an array because a walk hands a callback the `Item` its node sits in, and an item keeps
its identity while the list around it changes — so a callback can remove the node it was given, or
put another one beside it, and the walk goes on correctly.

```go
list := tree.List()          // the "children" field of a node, or nil
first := list.First()        // the first node, or nil
last := list.Last()          // the last node, or nil
n := list.Len()              // how many
empty := list.IsEmpty()
nodes := list.Slice()        // the nodes as a Go slice, in order
```

## Changing a list

```go
item := list.AppendData(node)        // add at the end, and get its place
list.PrependData(node)               // add at the front
list.InsertBefore(newItem, item)     // put one in front of another; nil appends
list.Remove(item)                    // take one out
```

`AppendData` and `PrependData` return the `*Item` the node now sits in, which is what
`InsertBefore` and `Remove` take.

## Walking a list

```go
list.Each(func(node *csstree.Node, item *csstree.Item, l *csstree.List) {
    // ...
})

list.EachRight(func(node *csstree.Node, item *csstree.Item, l *csstree.List) {
    // the same, from the back
})
```

Both read the next item before they call the callback, so the callback may remove the item it was
given without losing its place.

## Building one

```go
list := csstree.NewList()
list.FromSlice([]*csstree.Node{a, b, c})
```

An `Item` is `{ Data *Node; Prev, Next *Item }`, so a list can also be walked by hand from `Head`
or `Tail`.
