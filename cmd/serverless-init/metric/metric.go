// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package metric

import (
	"time"

	"github.com/DataDog/datadog-agent/pkg/aggregator"
	"github.com/DataDog/datadog-agent/pkg/metrics"
)

// AddColdStartMetric adds the coldstart metric to the demultiplexer
func AddColdStartMetric(tags []string, timestamp time.Time, demux aggregator.Demultiplexer) {
	add("gcp.run.enhanced.cold_start", tags, time.Now(), demux)
}

// AddShutdownMetric adds the shutdown metric to the demultiplexer
func AddShutdownMetric(tags []string, timestamp time.Time, demux aggregator.Demultiplexer) {
	add("gcp.run.enhanced.shutdown", tags, time.Now(), demux)
}

func add(name string, tags []string, timestamp time.Time, demux aggregator.Demultiplexer) {
	metricTimestamp := float64(timestamp.UnixNano()) / float64(time.Second)
	demux.AggregateSample(metrics.MetricSample{
		Name:       name,
		Value:      1.0,
		Mtype:      metrics.DistributionType,
		Tags:       tags,
		SampleRate: 1,
		Timestamp:  metricTimestamp,
	})
}
