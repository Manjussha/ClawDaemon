package queue

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/clawdaemon/internal/db"
)

func TestQueue_EnqueueDequeue(t *testing.T) {
	// Use temp file for SQLite.
	tmp := filepath.Join(os.TempDir(), "clawdaemon_test_queue.db")
	defer os.Remove(tmp)

	database, err := db.New(tmp)
	require.NoError(t, err)
	defer database.Close()
	require.NoError(t, database.Migrate())

	q := New(database)
	ctx := context.Background()

	// Enqueue a task.
	task := &db.Task{
		Title:    "Test Task",
		Prompt:   "Write a hello world program",
		Priority: 5,
	}
	id, err := q.Enqueue(ctx, task)
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))

	// Dequeue it.
	got, err := q.Dequeue(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Test Task", got.Title)
	assert.Equal(t, "running", got.Status)

	// Queue should now be empty.
	empty, err := q.Dequeue(ctx, 1)
	require.NoError(t, err)
	assert.Nil(t, empty)

	// Mark done.
	err = q.MarkDone(ctx, got.ID, "Hello, World!", "", 10, 6)
	require.NoError(t, err)
}

func TestQueue_MarkFailed(t *testing.T) {
	tmp := filepath.Join(os.TempDir(), "clawdaemon_test_fail.db")
	defer os.Remove(tmp)

	database, err := db.New(tmp)
	require.NoError(t, err)
	defer database.Close()
	require.NoError(t, database.Migrate())

	q := New(database)
	ctx := context.Background()

	id, err := q.Enqueue(ctx, &db.Task{Prompt: "fail me", Priority: 1})
	require.NoError(t, err)

	task, err := q.Dequeue(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, task)

	err = q.MarkFailed(ctx, int(id), "rate limit detected")
	require.NoError(t, err)

	got, err := q.GetTask(ctx, int(id))
	require.NoError(t, err)
	assert.Equal(t, "failed", got.Status)
	assert.Equal(t, "rate limit detected", got.ErrorMessage)
}
