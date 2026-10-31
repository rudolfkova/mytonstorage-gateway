package files

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	remotes "mytonstorage-gateway/pkg/clients/remote-ton-storage"
	tonstorageClient "mytonstorage-gateway/pkg/clients/ton-storage"
)

type fakeReports struct{}

func (fakeReports) HasBan(context.Context, string) (bool, error) {
	return false, nil
}

type fakeStorage struct {
	bag         *tonstorageClient.BagDetailed
	missing     bool
	addCalls    int
	downloadAll bool
}

func (f *fakeStorage) GetBag(context.Context, string) (*tonstorageClient.BagDetailed, error) {
	if f.missing || f.bag == nil {
		return nil, tonstorageClient.ErrNotFound
	}
	return f.bag, nil
}

func (f *fakeStorage) AddBag(_ context.Context, _ string, downloadAll bool) error {
	f.addCalls++
	f.downloadAll = downloadAll
	f.missing = false
	return nil
}

type fakeRemote struct {
	files       []tonstorageClient.File
	listCalls   atomic.Int32
	streamCalls atomic.Int32
}

func (f *fakeRemote) ListFiles(context.Context, string) (remotes.BagInfo, error) {
	f.listCalls.Add(1)
	return remotes.BagInfo{
		Files:      f.files,
		TotalSize:  30,
		PeersCount: 1,
	}, nil
}

func (f *fakeRemote) StreamFile(context.Context, string, string) (remotes.FileStream, error) {
	f.streamCalls.Add(1)
	return remotes.FileStream{
		FileStream: io.NopCloser(strings.NewReader("data")),
		Size:       4,
		PeersCount: 1,
	}, nil
}

func (f *fakeRemote) Close() {}

func testBag(root, dir string) *tonstorageClient.BagDetailed {
	return &tonstorageClient.BagDetailed{
		Bag: tonstorageClient.Bag{
			Description:  "coub",
			Size:         30,
			DirName:      dir,
			HeaderLoaded: true,
			Completed:    true,
		},
		Path: root,
		Files: []tonstorageClient.File{
			{Name: "index.html", Size: 10},
			{Name: "10boo9/poster.jpg", Size: 20},
		},
	}
}

func newTestService(st storage, remote remotes.Client) *service {
	return NewService(fakeReports{}, st, remote, slog.New(slog.DiscardHandler)).(*service)
}

func TestGetPathInfoHollowFileFallsBackToRemote(t *testing.T) {
	root := t.TempDir()
	st := &fakeStorage{bag: testBag(root, "bagdir")}
	remote := &fakeRemote{files: st.bag.Files}
	svc := newTestService(st, remote)

	info, err := svc.GetPathInfo(context.Background(), "bag", "index.html")
	if err != nil {
		t.Fatalf("GetPathInfo: %v", err)
	}
	if info.SingleFilePath != "" {
		t.Fatalf("SingleFilePath = %q, want empty", info.SingleFilePath)
	}
	if info.StreamFile == nil {
		t.Fatal("StreamFile is nil")
	}
	if st.addCalls != 0 {
		t.Fatalf("AddBag calls = %d, want 0", st.addCalls)
	}
	if remote.streamCalls.Load() != 1 {
		t.Fatalf("StreamFile calls = %d, want 1", remote.streamCalls.Load())
	}
}

func TestGetPathInfoLocalFileOnDisk(t *testing.T) {
	root := t.TempDir()
	dir := "bagdir"
	localPath := filepath.Join(root, dir, "index.html")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, []byte("html"), 0o644); err != nil {
		t.Fatal(err)
	}

	st := &fakeStorage{bag: testBag(root, dir)}
	remote := &fakeRemote{files: st.bag.Files}
	svc := newTestService(st, remote)

	info, err := svc.GetPathInfo(context.Background(), "bag", "index.html")
	if err != nil {
		t.Fatalf("GetPathInfo: %v", err)
	}
	if info.SingleFilePath != localPath {
		t.Fatalf("SingleFilePath = %q, want %q", info.SingleFilePath, localPath)
	}
	if info.StreamFile != nil {
		t.Fatal("StreamFile is set")
	}
	if st.addCalls != 0 {
		t.Fatalf("AddBag calls = %d, want 0", st.addCalls)
	}
	if remote.listCalls.Load() != 0 || remote.streamCalls.Load() != 0 {
		t.Fatalf("remote calls list=%d stream=%d, want 0", remote.listCalls.Load(), remote.streamCalls.Load())
	}
}

func TestGetPathInfoEnsureThenRemoteWhenFileMissing(t *testing.T) {
	root := t.TempDir()
	bag := testBag(root, "bagdir")
	st := &fakeStorage{bag: bag, missing: true}
	remote := &fakeRemote{files: bag.Files}
	svc := newTestService(st, remote)

	info, err := svc.GetPathInfo(context.Background(), "bag", "index.html")
	if err != nil {
		t.Fatalf("GetPathInfo: %v", err)
	}
	if st.addCalls != 1 {
		t.Fatalf("AddBag calls = %d, want 1", st.addCalls)
	}
	if st.downloadAll {
		t.Fatal("AddBag downloadAll = true, want false")
	}
	if info.SingleFilePath != "" {
		t.Fatalf("SingleFilePath = %q, want empty", info.SingleFilePath)
	}
	if info.StreamFile == nil {
		t.Fatal("StreamFile is nil")
	}
	if remote.streamCalls.Load() != 1 {
		t.Fatalf("StreamFile calls = %d, want 1", remote.streamCalls.Load())
	}
}

func TestGetPathInfoDirectoryListingWithoutData(t *testing.T) {
	root := t.TempDir()
	st := &fakeStorage{bag: testBag(root, "bagdir")}
	remote := &fakeRemote{files: st.bag.Files}
	svc := newTestService(st, remote)

	info, err := svc.GetPathInfo(context.Background(), "bag", "")
	if err != nil {
		t.Fatalf("GetPathInfo: %v", err)
	}
	if info.SingleFilePath != "" || info.StreamFile != nil {
		t.Fatalf("directory listing served as file: path=%q stream=%v", info.SingleFilePath, info.StreamFile != nil)
	}
	if len(info.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(info.Files))
	}
	if st.addCalls != 0 {
		t.Fatalf("AddBag calls = %d, want 0", st.addCalls)
	}
	if remote.listCalls.Load() != 0 || remote.streamCalls.Load() != 0 {
		t.Fatalf("remote calls list=%d stream=%d, want 0", remote.listCalls.Load(), remote.streamCalls.Load())
	}
}

type blockingStorage struct {
	mu          sync.Mutex
	bag         *tonstorageClient.BagDetailed
	addCalls    atomic.Int32
	addStarted  chan struct{}
	releaseAdd  chan struct{}
	headerReady bool
}

func (s *blockingStorage) GetBag(context.Context, string) (*tonstorageClient.BagDetailed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.headerReady {
		return nil, tonstorageClient.ErrNotFound
	}
	return s.bag, nil
}

func (s *blockingStorage) AddBag(context.Context, string, bool) error {
	if s.addCalls.Add(1) == 1 {
		close(s.addStarted)
	}
	<-s.releaseAdd
	s.mu.Lock()
	s.headerReady = true
	s.mu.Unlock()
	return nil
}

type missingStorage struct {
	addCalls atomic.Int32
}

func (s *missingStorage) GetBag(context.Context, string) (*tonstorageClient.BagDetailed, error) {
	return nil, tonstorageClient.ErrNotFound
}

func (s *missingStorage) AddBag(context.Context, string, bool) error {
	s.addCalls.Add(1)
	return nil
}

func TestGetPathInfoParallelEnsureOnce(t *testing.T) {
	root := t.TempDir()
	bag := testBag(root, "bagdir")
	st := &blockingStorage{
		bag:        bag,
		addStarted: make(chan struct{}),
		releaseAdd: make(chan struct{}),
	}
	remote := &fakeRemote{files: bag.Files}
	svc := newTestService(st, remote)

	const n = 8
	errCh := make(chan error, n)
	for range n {
		go func() {
			info, err := svc.GetPathInfo(context.Background(), "bag", "index.html")
			if err != nil {
				errCh <- err
				return
			}
			if info.StreamFile == nil {
				errCh <- context.Canceled
				return
			}
			errCh <- nil
		}()
	}

	select {
	case <-st.addStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("AddBag was not called")
	}
	close(st.releaseAdd)

	for i := 0; i < n; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("GetPathInfo: %v", err)
		}
	}
	if got := st.addCalls.Load(); got != 1 {
		t.Fatalf("AddBag calls = %d, want 1", got)
	}
}

func TestGetPathInfoHeaderFailCacheSkipsAddBag(t *testing.T) {
	st := &missingStorage{}
	remote := &fakeRemote{files: []tonstorageClient.File{{Name: "index.html", Size: 10}}}
	svc := newTestService(st, remote)
	svc.headerLoadTimeout = 40 * time.Millisecond
	svc.headerPollInterval = 5 * time.Millisecond
	svc.headerFailTTL = time.Hour

	if _, err := svc.GetPathInfo(context.Background(), "bag", "index.html"); err != nil {
		t.Fatalf("first GetPathInfo: %v", err)
	}
	if got := st.addCalls.Load(); got != 1 {
		t.Fatalf("AddBag calls after first = %d, want 1", got)
	}

	if _, err := svc.GetPathInfo(context.Background(), "bag", "index.html"); err != nil {
		t.Fatalf("second GetPathInfo: %v", err)
	}
	if got := st.addCalls.Load(); got != 1 {
		t.Fatalf("AddBag calls after cached fail = %d, want 1", got)
	}
}

func TestGetPathInfoCanceledWaiterDoesNotAbortFlight(t *testing.T) {
	root := t.TempDir()
	bag := testBag(root, "bagdir")
	st := &blockingStorage{
		bag:        bag,
		addStarted: make(chan struct{}),
		releaseAdd: make(chan struct{}),
	}
	remote := &fakeRemote{files: bag.Files}
	svc := newTestService(st, remote)
	svc.headerLoadTimeout = 2 * time.Second
	svc.headerPollInterval = 10 * time.Millisecond

	waiterCtx, cancelWaiter := context.WithCancel(context.Background())
	waiterDone := make(chan error, 1)
	go func() {
		_, err := svc.GetPathInfo(waiterCtx, "bag", "index.html")
		waiterDone <- err
	}()

	select {
	case <-st.addStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("AddBag was not called")
	}

	followerDone := make(chan error, 1)
	go func() {
		info, err := svc.GetPathInfo(context.Background(), "bag", "index.html")
		if err != nil {
			followerDone <- err
			return
		}
		if info.StreamFile == nil {
			followerDone <- context.Canceled
			return
		}
		followerDone <- nil
	}()

	cancelWaiter()
	select {
	case err := <-waiterDone:
		if err != nil {
			t.Fatalf("canceled waiter: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled waiter did not return")
	}

	close(st.releaseAdd)

	select {
	case err := <-followerDone:
		if err != nil {
			t.Fatalf("follower: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("follower did not return")
	}
	if got := st.addCalls.Load(); got != 1 {
		t.Fatalf("AddBag calls = %d, want 1", got)
	}
}
