package remotetonstorage

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	tonstorage "github.com/xssnick/tonutils-storage/storage"
)

func newTestClient(load func(context.Context, string) (*tonstorage.Torrent, tonstorage.TorrentDownloader, error)) *client {
	return &client{
		bagsCache:         NewBagsCache(8),
		loadTorrent:       load,
		downloaderTimeout: time.Second,
		torrentFailTTL:    time.Hour,
		flights:           make(map[string]*torrentFlight),
		failUntil:         make(map[string]torrentFail),
	}
}

func TestGetTorrentParallelLoadOnce(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	c := newTestClient(func(context.Context, string) (*tonstorage.Torrent, tonstorage.TorrentDownloader, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return nil, nil, ErrTimeout
	})

	const n = 8
	errCh := make(chan error, n)
	for range n {
		go func() {
			_, _, err := c.getTorrent(context.Background(), "bag")
			errCh <- err
		}()
	}

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("loadTorrent was not called")
	}
	close(release)

	for i := 0; i < n; i++ {
		if err := <-errCh; err != ErrTimeout {
			t.Fatalf("getTorrent: %v", err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("loadTorrent calls = %d, want 1", got)
	}
}

func TestGetTorrentFailCacheSkipsLoad(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(func(context.Context, string) (*tonstorage.Torrent, tonstorage.TorrentDownloader, error) {
		calls.Add(1)
		return nil, nil, ErrTimeout
	})

	if _, _, err := c.getTorrent(context.Background(), "bag"); err != ErrTimeout {
		t.Fatalf("first getTorrent: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls after first = %d, want 1", got)
	}

	if _, _, err := c.getTorrent(context.Background(), "bag"); err != ErrTimeout {
		t.Fatalf("second getTorrent: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("calls after cached fail = %d, want 1", got)
	}
}

func TestGetTorrentCanceledWaiterDoesNotAbortFlight(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	c := newTestClient(func(context.Context, string) (*tonstorage.Torrent, tonstorage.TorrentDownloader, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return nil, nil, ErrTimeout
	})

	waiterCtx, cancelWaiter := context.WithCancel(context.Background())
	waiterDone := make(chan error, 1)
	go func() {
		_, _, err := c.getTorrent(waiterCtx, "bag")
		waiterDone <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("loadTorrent was not called")
	}

	followerDone := make(chan error, 1)
	go func() {
		_, _, err := c.getTorrent(context.Background(), "bag")
		followerDone <- err
	}()

	time.Sleep(20 * time.Millisecond)
	cancelWaiter()
	select {
	case err := <-waiterDone:
		if err == nil {
			t.Fatal("canceled waiter returned nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled waiter did not return")
	}

	close(release)

	select {
	case err := <-followerDone:
		if err != ErrTimeout {
			t.Fatalf("follower: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("follower did not return")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("loadTorrent calls = %d, want 1", got)
	}
}
