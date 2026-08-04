package main

import "math"

// fileTransferParallelism models the maximum number of files whose network
// round trips can overlap on one dependency. Bandwidth is shared by the total
// volume; link latency is paid once per batch of files.
const fileTransferParallelism = 1

func dependencyTransferSeconds(dependency Dependency, bandwidthMBps, latencySeconds float64) float64 {
	fileCount := max(1, dependency.FileCount)
	batches := int(math.Ceil(float64(fileCount) / float64(fileTransferParallelism)))
	return dependency.DataMB/maxf(bandwidthMBps, 0.001) + float64(batches)*latencySeconds
}
