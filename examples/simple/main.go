package main

import (
	"context"
	"fmt"
	"time"

	"github.com/hatchet-dev/buffered"
)

type mockItem struct {
	ID    int
	Size  int
	Value string
}

type mockResult struct {
	ID int
}

func mockFlushFunc(ctx context.Context, items []mockItem) ([]mockResult, error) {
	var results []mockResult
	for _, item := range items {
		results = append(results, mockResult{ID: item.ID})
	}
	return results, nil
}

func mockSizeFunc(item mockItem) int {
	return item.Size
}

func main() {
	opts := buffered.BufferOpts[mockItem, mockResult]{
		Name:        "test",
		MaxCapacity: 2,
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

	doneChan, err := b.BuffItem(mockItem{
		ID:    1,
		Size:  10,
		Value: "one",
	})

	if err != nil {
		panic(err)
	}

	// This will return after 5 seconds
	resp := <-doneChan

	fmt.Println(resp.Result)
}
