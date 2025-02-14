package fileretrieval

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethersphere/beekeeper/pkg/bee"
	"github.com/ethersphere/beekeeper/pkg/beeclient/api"
	"github.com/ethersphere/beekeeper/pkg/random"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/prometheus/common/expfmt"
)

// Options represents pushsync check options
type Options struct {
	UploadNodeCount int
	FilesPerNode    int
	FileName        string
	FileSize        int64
	Seed            int64
	PostageAmount   int64
	PostageWait     time.Duration
}

var errFileRetrieval = errors.New("file retrieval")

// Check uploads files on cluster and downloads them from the last node in the cluster
func Check(c *bee.Cluster, o Options, pusher *push.Pusher, pushMetrics bool) (err error) {
	ctx := context.Background()
	rnds := random.PseudoGenerators(o.Seed, o.UploadNodeCount)
	fmt.Printf("Seed: %d\n", o.Seed)

	pusher.Collector(uploadedCounter)
	pusher.Collector(uploadTimeGauge)
	pusher.Collector(uploadTimeHistogram)
	pusher.Collector(downloadedCounter)
	pusher.Collector(downloadTimeGauge)
	pusher.Collector(downloadTimeHistogram)
	pusher.Collector(retrievedCounter)
	pusher.Collector(notRetrievedCounter)

	pusher.Format(expfmt.FmtText)

	overlays, err := c.FlattenOverlays(ctx)
	if err != nil {
		return err
	}

	clients, err := c.NodesClients(ctx)
	if err != nil {
		return err
	}

	sortedNodes := c.NodeNames()
	lastNodeName := sortedNodes[len(sortedNodes)-1]
	for i := 0; i < o.UploadNodeCount; i++ {
		nodeName := sortedNodes[i]
		for j := 0; j < o.FilesPerNode; j++ {
			file := bee.NewRandomFile(rnds[i], fmt.Sprintf("%s-%d-%d", o.FileName, i, j), o.FileSize)

			depth := 2 + bee.EstimatePostageBatchDepth(file.Size())
			batchID, err := clients[nodeName].CreatePostageBatch(ctx, o.PostageAmount, depth, "test-label")
			if err != nil {
				return fmt.Errorf("node %s: created batched id %w", nodeName, err)
			}

			fmt.Printf("node %s: created batched id %s\n", nodeName, batchID)

			time.Sleep(o.PostageWait)

			t0 := time.Now()

			client := clients[nodeName]
			if err := client.UploadFile(ctx, &file, api.UploadOptions{BatchID: batchID}); err != nil {
				return fmt.Errorf("node %s: %w", nodeName, err)
			}

			d0 := time.Since(t0)

			uploadedCounter.WithLabelValues(overlays[nodeName].String()).Inc()
			uploadTimeGauge.WithLabelValues(overlays[nodeName].String(), file.Address().String()).Set(d0.Seconds())
			uploadTimeHistogram.Observe(d0.Seconds())

			time.Sleep(1 * time.Second)
			t1 := time.Now()

			client = clients[lastNodeName]

			size, hash, err := client.DownloadFile(ctx, file.Address())
			if err != nil {
				return fmt.Errorf("node %s: %w", lastNodeName, err)
			}
			d1 := time.Since(t1)

			downloadedCounter.WithLabelValues(overlays[nodeName].String()).Inc()
			downloadTimeGauge.WithLabelValues(overlays[nodeName].String(), file.Address().String()).Set(d1.Seconds())
			downloadTimeHistogram.Observe(d1.Seconds())

			if !bytes.Equal(file.Hash(), hash) {
				notRetrievedCounter.WithLabelValues(overlays[nodeName].String()).Inc()
				fmt.Printf("Node %s. File %d not retrieved successfully. Uploaded size: %d Downloaded size: %d Node: %s File: %s\n", nodeName, j, file.Size(), size, overlays[nodeName].String(), file.Address().String())
				return errFileRetrieval
			}

			retrievedCounter.WithLabelValues(overlays[nodeName].String()).Inc()
			fmt.Printf("Node %s. File %d retrieved successfully. Node: %s File: %s\n", nodeName, j, overlays[nodeName].String(), file.Address().String())

			if pushMetrics {
				if err := pusher.Push(); err != nil {
					fmt.Printf("node %s: %v\n", nodeName, err)
				}
			}
		}
	}

	return
}

// CheckFull uploads files on cluster and downloads them from the all nodes in the cluster
func CheckFull(c *bee.Cluster, o Options, pusher *push.Pusher, pushMetrics bool) (err error) {
	ctx := context.Background()
	rnds := random.PseudoGenerators(o.Seed, o.UploadNodeCount)
	fmt.Printf("Seed: %d\n", o.Seed)

	pusher.Collector(uploadedCounter)
	pusher.Collector(uploadTimeGauge)
	pusher.Collector(uploadTimeHistogram)
	pusher.Collector(downloadedCounter)
	pusher.Collector(downloadTimeGauge)
	pusher.Collector(downloadTimeHistogram)
	pusher.Collector(retrievedCounter)
	pusher.Collector(notRetrievedCounter)

	pusher.Format(expfmt.FmtText)

	overlays, err := c.FlattenOverlays(ctx)
	if err != nil {
		return err
	}

	sortedNodes := c.NodeNames()

	clients, err := c.NodesClients(ctx)
	if err != nil {
		return err
	}

	for i := 0; i < o.UploadNodeCount; i++ {
		nodeName := sortedNodes[i]
		for j := 0; j < o.FilesPerNode; j++ {
			file := bee.NewRandomFile(rnds[i], fmt.Sprintf("%s-%d-%d", o.FileName, i, j), o.FileSize)

			depth := 2 + bee.EstimatePostageBatchDepth(file.Size())
			batchID, err := clients[nodeName].CreatePostageBatch(ctx, o.PostageAmount, depth, "test-label")
			if err != nil {
				return fmt.Errorf("node %s: created batched id %w", nodeName, err)
			}

			fmt.Printf("node %s: created batched id %s\n", nodeName, batchID)

			time.Sleep(o.PostageWait)

			t0 := time.Now()
			if err := clients[nodeName].UploadFile(ctx, &file, api.UploadOptions{BatchID: batchID}); err != nil {
				return fmt.Errorf("node %s: %w", nodeName, err)
			}
			d0 := time.Since(t0)

			uploadedCounter.WithLabelValues(overlays[nodeName].String()).Inc()
			uploadTimeGauge.WithLabelValues(overlays[nodeName].String(), file.Address().String()).Set(d0.Seconds())
			uploadTimeHistogram.Observe(d0.Seconds())

			time.Sleep(1 * time.Second)
			for n, nc := range clients {
				if n == nodeName {
					continue
				}

				t1 := time.Now()
				size, hash, err := nc.DownloadFile(ctx, file.Address())
				if err != nil {
					return fmt.Errorf("node %s: %w", n, err)
				}
				d1 := time.Since(t1)

				downloadedCounter.WithLabelValues(overlays[nodeName].String()).Inc()
				downloadTimeGauge.WithLabelValues(overlays[nodeName].String(), file.Address().String()).Set(d1.Seconds())
				downloadTimeHistogram.Observe(d1.Seconds())

				if !bytes.Equal(file.Hash(), hash) {
					notRetrievedCounter.WithLabelValues(overlays[nodeName].String()).Inc()
					fmt.Printf("Node %s. File %d not retrieved successfully from node %s. Uploaded size: %d Downloaded size: %d Node: %s Download node: %s File: %s\n", nodeName, j, n, file.Size(), size, overlays[nodeName].String(), overlays[n].String(), file.Address().String())
					return errFileRetrieval
				}

				retrievedCounter.WithLabelValues(overlays[nodeName].String()).Inc()
				fmt.Printf("Node %s. File %d retrieved successfully from node %s. Node: %s Download node: %s File: %s\n", nodeName, j, n, overlays[nodeName].String(), overlays[n].String(), file.Address().String())

				if pushMetrics {
					if err := pusher.Push(); err != nil {
						fmt.Printf("node %s: %v\n", nodeName, err)
					}
				}
			}
		}
	}

	return
}
