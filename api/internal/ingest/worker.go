// Package ingest provides the document-ingestion queue and worker. Uploads are
// enqueued in Redis and processed asynchronously: load text -> AI chunk+embed
// -> update document status.
package ingest

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thefcan/ragdesk/api/internal/ai"
	"github.com/thefcan/ragdesk/api/internal/store"
)

const queueKey = "ragdesk:ingest"

// Queue enqueues documents for ingestion.
type Queue struct {
	rdb *redis.Client
}

// NewQueue returns a Queue backed by Redis.
func NewQueue(rdb *redis.Client) *Queue { return &Queue{rdb: rdb} }

// Enqueue schedules a document for ingestion.
func (q *Queue) Enqueue(ctx context.Context, documentID string) error {
	return q.rdb.LPush(ctx, queueKey, documentID).Err()
}

// Worker consumes the ingestion queue.
type Worker struct {
	rdb   *redis.Client
	store *store.Store
	ai    *ai.Client
	log   *slog.Logger
}

// NewWorker builds a Worker.
func NewWorker(rdb *redis.Client, st *store.Store, aiClient *ai.Client, log *slog.Logger) *Worker {
	return &Worker{rdb: rdb, store: st, ai: aiClient, log: log}
}

// Run consumes the queue until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	w.log.Info("ingest worker started")
	for {
		if ctx.Err() != nil {
			w.log.Info("ingest worker stopped")
			return
		}
		res, err := w.rdb.BRPop(ctx, 5*time.Second, queueKey).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue // idle timeout
			}
			if ctx.Err() != nil {
				w.log.Info("ingest worker stopped")
				return
			}
			w.log.Warn("ingest dequeue", slog.Any("err", err))
			time.Sleep(time.Second)
			continue
		}
		w.Process(ctx, res[1])
	}
}

// Process ingests a single document, updating its status.
func (w *Worker) Process(ctx context.Context, documentID string) {
	workspaceID, text, err := w.store.DocumentText(ctx, documentID)
	if err != nil {
		w.log.Error("ingest: load document", slog.String("document_id", documentID), slog.Any("err", err))
		return
	}
	count, err := w.ai.Ingest(ctx, documentID, workspaceID, text)
	if err != nil {
		w.log.Error("ingest: ai", slog.String("document_id", documentID), slog.Any("err", err))
		if mErr := w.store.MarkDocumentFailed(ctx, documentID, "ingestion failed"); mErr != nil {
			w.log.Error("ingest: mark failed", slog.Any("err", mErr))
		}
		return
	}
	if err := w.store.MarkDocumentReady(ctx, documentID, count); err != nil {
		w.log.Error("ingest: mark ready", slog.Any("err", err))
		return
	}
	w.log.Info("ingest complete", slog.String("document_id", documentID), slog.Int("chunks", count))
}
