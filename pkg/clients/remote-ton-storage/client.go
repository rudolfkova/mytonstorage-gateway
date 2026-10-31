// This code is copypasted from the tonutils-proxy package and modified.
// Original package:
// https://github.com/xssnick/Tonutils-Proxy
// According to the license, this code is licensed under the Apache License 2.0
// See the LICENSE file in the original package for more details.
package remotetonstorage

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/xssnick/tonutils-go/adnl"
	"github.com/xssnick/tonutils-go/adnl/dht"
	"github.com/xssnick/tonutils-go/liteclient"
	tonstorage "github.com/xssnick/tonutils-storage/storage"

	tonapi "mytonstorage-gateway/pkg/clients/ton-storage"
	"mytonstorage-gateway/pkg/constants"
)

var ErrNotFound = errors.New("not found")
var ErrTimeout = errors.New("timeout")

type Client interface {
	StreamFile(ctx context.Context, bagID, path string) (s FileStream, err error)
	ListFiles(ctx context.Context, bagID string) (BagInfo, error)
	Close()
}

type BagInfo struct {
	Description string
	TotalSize   uint64
	PeersCount  int
	Files       []tonapi.File
}

type FileStream struct {
	FileStream io.ReadCloser
	Size       uint64
	PeersCount int
}

type torrentFlight struct {
	done       chan struct{}
	torrent    *tonstorage.Torrent
	downloader tonstorage.TorrentDownloader
	err        error
}

type torrentFail struct {
	until time.Time
	err   error
}

type client struct {
	netMgr     adnl.NetManager
	dhtGateway *adnl.Gateway
	dhtClient  *dht.Client

	bagsCache *BagsCache

	loadTorrent       func(ctx context.Context, bagID string) (*tonstorage.Torrent, tonstorage.TorrentDownloader, error)
	downloaderTimeout time.Duration
	torrentFailTTL    time.Duration
	flightMu          sync.Mutex
	flights           map[string]*torrentFlight
	failMu            sync.Mutex
	failUntil         map[string]torrentFail

	metrics *RemoteTONStorageMetrics

	storageKey  ed25519.PrivateKey
	storageGate *adnl.Gateway
	srv         *tonstorage.Server
	conn        *tonstorage.Connector

	store *VirtualStorage
}

func (c *client) StreamFile(ctx context.Context, bagID, path string) (FileStream, error) {
	start := time.Now()
	torrent, downloader, err := c.getTorrent(ctx, bagID)
	if err != nil {
		if errors.Is(err, ErrTimeout) && torrent != nil {
			peers := torrent.GetPeers()
			if c.metrics != nil {
				c.metrics.streamFileReqs.WithLabelValues("timeout").Inc()
			}
			return FileStream{PeersCount: len(peers)}, ErrTimeout
		}
		if c.metrics != nil {
			c.metrics.streamFileReqs.WithLabelValues("error").Inc()
		}
		return FileStream{}, err
	}

	fileInfo, err := torrent.GetFileOffsets(path)
	if err != nil {
		if c.metrics != nil {
			c.metrics.streamFileReqs.WithLabelValues("not_found").Inc()
		}
		return FileStream{}, err
	}

	pieces := make([]uint32, 0, (fileInfo.ToPiece-fileInfo.FromPiece)+1)
	for p := fileInfo.FromPiece; p <= fileInfo.ToPiece; p++ {
		pieces = append(pieces, p)
	}

	fetch := tonstorage.NewPreFetcher(ctx, torrent, downloader, func(event tonstorage.Event) {}, 64, pieces)
	pr, pw := io.Pipe()

	// metrics wrapping reader
	var firstByteOnce sync.Once
	if c.metrics != nil {
		c.metrics.activeStreams.Inc()
	}

	wrapped := &meteredStream{
		ReadCloser:    pr,
		start:         start,
		metrics:       c.metrics,
		firstByteOnce: &firstByteOnce,
		ttfbObserved:  false,
	}

	go func(ctx context.Context) {
		defer fetch.Stop()
		defer pw.Close()

		for p := fileInfo.FromPiece; p <= fileInfo.ToPiece; p++ {
			select {
			case <-ctx.Done():
				_ = pw.CloseWithError(ctx.Err())
				return
			default:
			}

			data, _, err := fetch.Get(ctx, p)
			if err != nil {
				_ = pw.CloseWithError(fmt.Errorf("failed to download piece %d: %w", p, err))
				return
			}
			part := data
			if p == fileInfo.ToPiece {
				part = part[:fileInfo.ToPieceOffset]
			}
			if p == fileInfo.FromPiece {
				part = part[fileInfo.FromPieceOffset:]
			}
			if len(part) == 0 {
				continue
			}
			if _, err = pw.Write(part); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}
	}(ctx)

	if c.metrics != nil {
		c.metrics.streamFileReqs.WithLabelValues("success").Inc()
	}

	peers := torrent.GetPeers()
	return FileStream{
		FileStream: wrapped,
		Size:       fileInfo.Size,
		PeersCount: len(peers),
	}, nil
}

// ListFiles returns all files in the bag with sizes by loading the torrent header via ADNL.
func (c *client) ListFiles(ctx context.Context, bagID string) (info BagInfo, err error) {
	start := time.Now()
	defer func() {
		result := "success"
		if err != nil {
			result = "error"
			if errors.Is(err, ErrTimeout) {
				result = "timeout"
			}
		}

		if c.metrics != nil {
			c.metrics.listFilesReqs.WithLabelValues(result).Inc()
			c.metrics.listFilesDuration.WithLabelValues(result).Observe(time.Since(start).Seconds())
		}
	}()

	torrent, _, err := c.getTorrent(ctx, bagID)
	if err != nil {
		if errors.Is(err, ErrTimeout) && torrent != nil {
			peers := torrent.GetPeers()
			return BagInfo{
				PeersCount: len(peers),
			}, ErrTimeout
		}

		return BagInfo{}, fmt.Errorf("failed to get torrent: %w", err)
	}

	files := make([]tonapi.File, 0, torrent.Header.FilesCount)
	for i := uint32(0); i < torrent.Header.FilesCount; i++ {
		info, err := torrent.GetFileOffsetsByID(i)
		if err != nil {
			return BagInfo{}, fmt.Errorf("failed to read file %d: %w", i, err)
		}
		files = append(files, tonapi.File{
			Index: info.Index,
			Name:  info.Name,
			Size:  info.Size,
		})
	}

	return BagInfo{
		TotalSize:   torrent.Info.FileSize,
		Description: torrent.Info.Description.Value,
		PeersCount:  len(torrent.GetPeers()),
		Files:       files,
	}, nil
}

func (c *client) Close() {
	if c == nil {
		return
	}
	if c.srv != nil {
		c.srv.Stop()
	}
	if c.storageGate != nil {
		c.storageGate.Close()
	}
	if c.dhtClient != nil {
		c.dhtClient.Close()
	}
	if c.dhtGateway != nil {
		c.dhtGateway.Close()
	}
	if c.netMgr != nil {
		c.netMgr.Close()
	}
}

func (c *client) getTorrent(ctx context.Context, bagID string) (torrent *tonstorage.Torrent, downloader tonstorage.TorrentDownloader, err error) {
	if t, d, ok := c.bagsCache.Get(bagID); ok {
		return t, d, nil
	}

	if failErr, ok := c.torrentRecentlyFailed(bagID); ok {
		return nil, nil, failErr
	}

	flight, leader := c.getOrStartTorrentFlight(bagID)
	if leader {
		go c.runTorrentFlight(flight, bagID)
	}

	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-flight.done:
		return flight.torrent, flight.downloader, flight.err
	}
}

func (c *client) runTorrentFlight(flight *torrentFlight, bagID string) {
	loadCtx, cancel := context.WithTimeout(context.Background(), c.downloaderTimeout)
	defer cancel()

	load := c.loadTorrent
	if load == nil {
		load = c.doLoadTorrent
	}

	t, d, err := load(loadCtx, bagID)
	flight.torrent = t
	flight.downloader = d
	flight.err = err
	if err != nil {
		c.rememberTorrentFail(bagID, err)
	}
	close(flight.done)
	c.dropTorrentFlight(bagID, flight)
}

func (c *client) getOrStartTorrentFlight(bagID string) (*torrentFlight, bool) {
	c.flightMu.Lock()
	defer c.flightMu.Unlock()

	if f, ok := c.flights[bagID]; ok {
		return f, false
	}

	f := &torrentFlight{done: make(chan struct{})}
	c.flights[bagID] = f
	return f, true
}

func (c *client) dropTorrentFlight(bagID string, flight *torrentFlight) {
	c.flightMu.Lock()
	defer c.flightMu.Unlock()
	if c.flights[bagID] == flight {
		delete(c.flights, bagID)
	}
}

func (c *client) torrentRecentlyFailed(bagID string) (error, bool) {
	c.failMu.Lock()
	defer c.failMu.Unlock()

	entry, ok := c.failUntil[bagID]
	if !ok {
		return nil, false
	}
	if time.Now().Before(entry.until) {
		return entry.err, true
	}
	delete(c.failUntil, bagID)
	return nil, false
}

func (c *client) rememberTorrentFail(bagID string, err error) {
	c.failMu.Lock()
	defer c.failMu.Unlock()
	c.failUntil[bagID] = torrentFail{
		until: time.Now().Add(c.torrentFailTTL),
		err:   err,
	}
}

func (c *client) doLoadTorrent(ctx context.Context, bagID string) (torrent *tonstorage.Torrent, downloader tonstorage.TorrentDownloader, err error) {
	id, err := hex.DecodeString(bagID)
	if err != nil {
		err = fmt.Errorf("invalid bag id hex: %w", err)
		return
	}

	torrent = tonstorage.NewTorrent("", c.store, c.conn)
	torrent.BagID = id
	_ = c.store.SetTorrent(torrent)

	if err = torrent.Start(true, false, false); err != nil {
		torrent.Stop()
		err = fmt.Errorf("failed to start torrent: %w", err)
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, c.downloaderTimeout)
	defer cancel()

	dStart := time.Now()
	downloader, err = c.conn.CreateDownloader(timeoutCtx, torrent)
	if err != nil {
		torrent.Stop()
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout") {
			err = ErrTimeout
			if c.metrics != nil {
				c.metrics.downloaderCreations.WithLabelValues("timeout").Inc()
				c.metrics.downloaderCreationDuration.WithLabelValues("timeout").Observe(time.Since(dStart).Seconds())
			}
			return
		}
		if c.metrics != nil {
			c.metrics.downloaderCreations.WithLabelValues("error").Inc()
			c.metrics.downloaderCreationDuration.WithLabelValues("error").Observe(time.Since(dStart).Seconds())
		}
		err = fmt.Errorf("failed to create downloader: %w", err)
		return
	}
	if c.metrics != nil {
		c.metrics.downloaderCreations.WithLabelValues("success").Inc()
		c.metrics.downloaderCreationDuration.WithLabelValues("success").Observe(time.Since(dStart).Seconds())
	}

	if torrent.Header == nil || torrent.Info == nil {
		torrent.Stop()
		err = fmt.Errorf("torrent header or info not loaded")
		return
	}

	c.bagsCache.Set(bagID, torrent, downloader)

	return
}

func NewClient(ctx context.Context, configURL string, cache *BagsCache, metrics *RemoteTONStorageMetrics) (Client, error) {
	if configURL == "" {
		configURL = "https://ton-blockchain.github.io/global.config.json"
	}
	cfg, err := liteclient.GetConfigFromUrl(ctx, configURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch TON global config: %w", err)
	}

	dl, err := adnl.DefaultListener(":")
	if err != nil {
		return nil, fmt.Errorf("failed to create adnl listener: %w", err)
	}
	netMgr := adnl.NewMultiNetReader(dl)

	_, dhtKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		netMgr.Close()
		return nil, fmt.Errorf("failed to generate dht key: %w", err)
	}
	dhtGateway := adnl.NewGatewayWithNetManager(dhtKey, netMgr)
	if err = dhtGateway.StartClient(); err != nil {
		netMgr.Close()
		return nil, fmt.Errorf("failed to start dht gateway: %w", err)
	}

	dhtClient, err := dht.NewClientFromConfig(dhtGateway, cfg)
	if err != nil {
		dhtGateway.Close()
		netMgr.Close()
		return nil, fmt.Errorf("failed to init dht client: %w", err)
	}

	_, storageKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		dhtClient.Close()
		dhtGateway.Close()
		netMgr.Close()
		return nil, fmt.Errorf("failed to generate storage key: %w", err)
	}
	storageGate := adnl.NewGatewayWithNetManager(storageKey, netMgr)

	listenThreads := 1
	if err = storageGate.StartClient(listenThreads); err != nil {
		dhtClient.Close()
		dhtGateway.Close()
		netMgr.Close()
		return nil, fmt.Errorf("failed to start storage gateway: %w", err)
	}

	store := NewVirtualStorage()
	srv := tonstorage.NewServer(dhtClient, storageGate, storageKey, false, 1)
	srv.SetStorage(store)
	conn := tonstorage.NewConnector(srv)

	if metrics != nil {
		cache.WithMetrics(metrics)
	}

	return &client{
		bagsCache:         cache,
		netMgr:            netMgr,
		dhtGateway:        dhtGateway,
		dhtClient:         dhtClient,
		storageKey:        storageKey,
		storageGate:       storageGate,
		srv:               srv,
		conn:              conn,
		store:             store,
		metrics:           metrics,
		downloaderTimeout: constants.RemoteDownloaderTimeout,
		torrentFailTTL:    constants.RemoteTorrentFailCacheTTL,
		flights:           make(map[string]*torrentFlight),
		failUntil:         make(map[string]torrentFail),
	}, nil
}
