package buffered

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestBufferInitialization(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	opts := BufferOpts[mockItem, mockResult]{
		Name:               "test",
		MaxCapacity:        5,
		FlushPeriod:        1 * time.Second,
		MaxDataSizeInQueue: 100,
		FlushFunc:          mockFlushFunc,
		SizeFunc:           mockSizeFunc,
		L:                  &logger,
	}

	// Initialize the buffer
	buf := NewBuffer(opts)
	assert.Equal(t, 5, buf.maxCapacity)
	assert.Equal(t, 1*time.Second, buf.flushPeriod)
	assert.Equal(t, 100, buf.maxDataSizeInQueue)
	assert.NotNil(t, buf.inputChan)
	assert.Equal(t, 0, buf.safeFetchSizeOfData())
	assert.Equal(t, initialized, buf.state)
	assert.NoError(t, Validate(opts))
}

func TestBufferValidation(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	opts := BufferOpts[mockItem, mockResult]{
		Name:               "test",
		MaxCapacity:        0,
		FlushPeriod:        -1 * time.Second,
		MaxDataSizeInQueue: -1,
		FlushFunc:          nil,
		SizeFunc:           nil,
		L:                  &logger,
	}

	require.Error(t, Validate(opts))
}

func TestBufferBuffering(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	opts := BufferOpts[mockItem, mockResult]{
		Name:               "test",
		MaxCapacity:        2,
		FlushPeriod:        1 * time.Second,
		MaxDataSizeInQueue: 100,
		FlushFunc:          mockFlushFunc,
		SizeFunc:           mockSizeFunc,
		L:                  &logger,
	}

	buf := NewBuffer(opts)
	_, err := buf.Start()

	require.NoError(t, err)

	item := mockItem{ID: 1, Size: 10, Value: "test"}
	doneChan, err := buf.BuffItem(item)
	require.NoError(t, err)

	resp := <-doneChan
	assert.NoError(t, resp.Err)
	assert.Equal(t, 1, resp.Result.ID)

	assert.Equal(t, 0, buf.safeFetchSizeOfData())
	assert.Equal(t, 0, buf.safeCheckSizeOfBuffer())

}

func TestBufferAutoFlushOnCapacity(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	opts := BufferOpts[mockItem, mockResult]{
		Name:               "test",
		MaxCapacity:        2,
		FlushPeriod:        5 * time.Second,
		MaxDataSizeInQueue: 100,
		FlushFunc:          mockFlushFunc,
		SizeFunc:           mockSizeFunc,
		L:                  &logger,
	}

	buf := NewBuffer(opts)
	_, err := buf.Start()

	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)

	for i := 1; i <= 2; i++ {
		go func(i int) {
			defer wg.Done()
			item := mockItem{ID: i, Size: 10, Value: "test"}
			doneChan, err := buf.BuffItem(item)
			require.NoError(t, err)

			resp := <-doneChan
			assert.NoError(t, resp.Err)
			assert.Equal(t, i, resp.Result.ID)
		}(i)
	}

	wg.Wait()

	assert.Equal(t, 0, buf.safeFetchSizeOfData())
	assert.Equal(t, 0, buf.safeCheckSizeOfBuffer())

}

func TestBufferAutoFlushOnSize(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	opts := BufferOpts[mockItem, mockResult]{
		Name:               "test",
		MaxCapacity:        10,
		FlushPeriod:        5 * time.Second,
		MaxDataSizeInQueue: 20, // Flush on size
		FlushFunc:          mockFlushFunc,
		SizeFunc:           mockSizeFunc,
		L:                  &logger,
	}

	buf := NewBuffer(opts)
	_, err := buf.Start()

	require.NoError(t, err)

	item := mockItem{ID: 1, Size: 25, Value: "test"}
	doneChan, err := buf.BuffItem(item)
	require.NoError(t, err)

	resp := <-doneChan
	assert.NoError(t, resp.Err)
	assert.Equal(t, 1, resp.Result.ID)

	assert.Equal(t, 0, buf.safeFetchSizeOfData())
	assert.Equal(t, 0, buf.safeCheckSizeOfBuffer())

}

func TestBufferTimeoutFlush(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	opts := BufferOpts[mockItem, mockResult]{
		Name:               "test",
		MaxCapacity:        10,
		FlushPeriod:        100 * time.Millisecond,
		MaxDataSizeInQueue: 100,
		FlushFunc:          mockFlushFunc,
		SizeFunc:           mockSizeFunc,
		L:                  &logger,
	}

	buf := NewBuffer(opts)
	_, err := buf.Start()

	require.NoError(t, err)

	item := mockItem{ID: 1, Size: 1, Value: "test"}
	doneChan, err := buf.BuffItem(item)
	require.NoError(t, err)

	select {
	case resp := <-doneChan:
		assert.NoError(t, resp.Err)
		assert.Equal(t, 1, resp.Result.ID)
	case <-time.After(500 * time.Millisecond):
		t.Error("Flush should have been triggered by timeout")
	}

	assert.Equal(t, 0, buf.safeFetchSizeOfData())
	assert.Equal(t, 0, buf.safeCheckSizeOfBuffer())
}

func TestBufferOrderPreservation(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)

	opts := BufferOpts[mockItem, mockResult]{
		Name:               "test",
		MaxCapacity:        5,
		FlushPeriod:        5 * time.Second,
		MaxDataSizeInQueue: 100,
		FlushFunc: func(ctx context.Context, items []mockItem) ([]mockResult, error) {
			var results []mockResult
			for _, item := range items {
				results = append(results, mockResult{ID: item.ID})
			}
			return results, nil
		},
		SizeFunc: mockSizeFunc,
		L:        &logger,
	}

	buf := NewBuffer(opts)
	_, err := buf.Start()

	require.NoError(t, err)

	var wg sync.WaitGroup
	expectedOrder := []int{1011, 20200, 33020, 4010221, 51}

	rand.Shuffle(len(expectedOrder), func(i, j int) {
		expectedOrder[i], expectedOrder[j] = expectedOrder[j], expectedOrder[i]
	})

	for _, id := range expectedOrder {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			item := mockItem{ID: id, Size: 10, Value: fmt.Sprintf("test-%d", id)}
			doneChan, err := buf.BuffItem(item)
			require.NoError(t, err)

			resp := <-doneChan
			require.NoError(t, resp.Err)

			assert.Equal(t, id, resp.Result.ID)
		}(id)
	}

	wg.Wait()

}
