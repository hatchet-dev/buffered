## Buffered writes in Go

`buffered` is a package for buffered writing to a database. It's agnostic to the type of database used. It supports different flushing strategies based on:

1. The maximum amount of time an item should spend in the buffer
2. The maximum size of the buffer
3. The maximum amount of memory in the buffer

This library is used internally by [Hatchet](https://github.com/hatchet-dev/hatchet), but is meant to be general-purpose. Issues and PRs are welcome.

## Usage

Install the package:

```
go get github.com/hatchet-dev/buffered
```

Basic usage is as follows:

```go
type mockItem struct {
	ID    int
	Size  int
	Value string
}

type mockResult struct {
	ID int
}

opts := buffered.BufferOpts[mockItem, mockResult]{
    Name:               "test",
    MaxCapacity:        2,
    // We set the flush period to be 5 seconds
    FlushPeriod:        5 * time.Second,
    MaxDataSizeInQueue: 100,
    FlushFunc:          mockFlushFunc,
    SizeFunc:           mockSizeFunc,
}

b := buffered.NewBuffer(opts)

cleanup, err := b.Start()
defer cleanup()

if err != nil {
    panic(err)
}

b.BuffItem()
doneChan, err := buf.BuffItem(mockItem{
    ID: 1,
    Size: 10,
    Value: "one"
})

if err != nil {
    panic(err)
}

// This will return after 5 seconds
resp := <-doneChan

fmt.Println(resp.Result)
```
