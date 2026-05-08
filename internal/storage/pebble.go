package storage

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/henrikrexed/semconv-proxy/internal/dictionary"
)

type Persister struct {
	db        *pebble.DB
	batchCh   chan *dictionary.AttributeEntry
	logger    *slog.Logger
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	interval  time.Duration
	batchSize int
}

func NewPersister(dataDir string, interval time.Duration, batchSize int, logger *slog.Logger) (*Persister, error) {
	opts := &pebble.Options{}
	db, err := pebble.Open(dataDir, opts)
	if err != nil {
		return nil, fmt.Errorf("storage: open pebble: %w", err)
	}
	return &Persister{
		db:        db,
		batchCh:   make(chan *dictionary.AttributeEntry, batchSize*2),
		logger:    logger,
		interval:  interval,
		batchSize: batchSize,
	}, nil
}

func (p *Persister) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	p.wg.Add(1)
	go p.drainLoop(ctx)
}

func (p *Persister) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	if p.db != nil {
		_ = p.db.Flush()
		_ = p.db.Close()
	}
}

func (p *Persister) Enqueue(entry *dictionary.AttributeEntry) {
	select {
	case p.batchCh <- entry:
	default:
		p.logger.Warn("persist channel full, dropping entry", "name", entry.Name)
	}
}

func (p *Persister) LoadAll() ([]*dictionary.AttributeEntry, error) {
	var entries []*dictionary.AttributeEntry
	iter, err := p.db.NewIter(nil)
	if err != nil {
		return nil, fmt.Errorf("storage: new iter: %w", err)
	}
	defer func() { _ = iter.Close() }()

	for iter.First(); iter.Valid(); iter.Next() {
		val, err := iter.ValueAndErr()
		if err != nil {
			continue
		}
		entry, err := BytesToEntry(val)
		if err != nil {
			p.logger.Warn("failed to unmarshal entry", "error", err)
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (p *Persister) drainLoop(ctx context.Context) {
	defer p.wg.Done()
	batch := make([]*dictionary.AttributeEntry, 0, p.batchSize)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.writeBatch(batch)
			return
		case entry := <-p.batchCh:
			batch = append(batch, entry)
			if len(batch) >= p.batchSize {
				p.writeBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				p.writeBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (p *Persister) writeBatch(entries []*dictionary.AttributeEntry) {
	if len(entries) == 0 {
		return
	}
	batch := p.db.NewBatch()
	for _, entry := range entries {
		signalType := "unknown"
		if len(entry.SignalTypes) > 0 {
			signalType = string(entry.SignalTypes[0])
		}
		key := StorageKey{SignalType: signalType, AttributeName: entry.Name}
		data, err := EntryToBytes(entry)
		if err != nil {
			p.logger.Error("failed to marshal entry", "error", err)
			continue
		}
		if err := batch.Set([]byte(key.String()), data, nil); err != nil {
			p.logger.Error("failed to set entry in batch", "error", err)
		}
	}
	if err := batch.Commit(nil); err != nil {
		p.logger.Error("failed to commit batch", "error", err)
	}
	if err := p.db.Flush(); err != nil {
		p.logger.Error("failed to flush", "error", err)
	}
}
