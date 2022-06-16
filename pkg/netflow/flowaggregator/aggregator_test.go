package flowaggregator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/DataDog/datadog-agent/pkg/aggregator/mocksender"
	"github.com/DataDog/datadog-agent/pkg/netflow/common"
	"github.com/DataDog/datadog-agent/pkg/netflow/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAggregator(t *testing.T) {
	stoppedMu := sync.RWMutex{} // Mutex needed to avoid race condition in test

	sender := mocksender.NewMockSender("")
	sender.On("MonotonicCount", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	sender.On("EventPlatformEvent", mock.Anything, mock.Anything).Return()
	sender.On("Commit").Return()
	conf := config.NetflowConfig{
		StopTimeout:             10,
		AggregatorBufferSize:    20,
		AggregatorFlushInterval: 1,
		LogPayloads:             true,
		Listeners: []config.ListenerConfig{
			{
				FlowType: common.TypeNetFlow9,
				BindHost: "127.0.0.1",
				Port:     uint16(1234),
				Workers:  10,
			},
		},
	}
	flow := &common.Flow{
		Namespace:      "my-ns",
		FlowType:       common.TypeNetFlow9,
		ExporterAddr:   []byte{127, 0, 0, 1},
		StartTimestamp: 1234568,
		EndTimestamp:   1234569,
		Bytes:          20,
		Packets:        4,
		SrcAddr:        []byte{10, 10, 10, 10},
		DstAddr:        []byte{10, 10, 10, 20},
		IPProtocol:     uint32(6),
		SrcPort:        uint32(2000),
		DstPort:        uint32(80),
		TCPFlags:       19,
		EtherType:      uint32(0x0800),
	}

	aggregator := NewFlowAggregator(sender, &conf, "my-hostname")
	aggregator.flushInterval = 1 * time.Second
	inChan := aggregator.GetFlowInChan()

	expectStartExisted := false
	go func() {
		aggregator.Start()
		stoppedMu.Lock()
		expectStartExisted = true
		stoppedMu.Unlock()
	}()
	inChan <- flow

	// language=json
	event := []byte(`
{
  "type": "netflow9",
  "sampling_rate": 0,
  "direction": "ingress",
  "start": 1234568,
  "end": 1234569,
  "bytes": 20,
  "packets": 4,
  "ether_type": "IPv4",
  "ip_protocol": "TCP",
  "exporter": {
    "ip": "127.0.0.1"
  },
  "source": {
    "ip": "10.10.10.10",
    "port": 2000,
    "mac": "00:00:00:00:00:00",
    "mask": "0.0.0.0/0"
  },
  "destination": {
    "ip": "10.10.10.20",
    "port": 80,
    "mac": "00:00:00:00:00:00",
    "mask": "0.0.0.0/0"
  },
  "ingress": {
    "interface": {
      "index": 0
    }
  },
  "egress": {
    "interface": {
      "index": 0
    }
  },
  "namespace": "my-ns",
  "host": "my-hostname",
  "tcp_flags": [
    "FIN",
    "SYN",
    "ACK"
  ],
  "next_hop": {
    "ip": ""
  }
}
`)
	compactEvent := new(bytes.Buffer)
	err := json.Compact(compactEvent, event)
	assert.NoError(t, err)

	err = waitForFlowsToBeFlushed(aggregator, 10*time.Second, 1)
	assert.NoError(t, err)

	sender.AssertEventPlatformEvent(t, compactEvent.String(), "network-devices-netflow")
	sender.AssertMetric(t, "MonotonicCount", "datadog.netflow.aggregator.flows_flushed", 1, "", nil)
	sender.AssertMetric(t, "MonotonicCount", "datadog.netflow.aggregator.flows_received", 1, "", nil)

	// Test aggregator Stop
	assert.False(t, expectStartExisted)
	aggregator.Stop()

	waitStopTimeout := time.After(2 * time.Second)
	waitStopTick := time.Tick(100 * time.Millisecond)
stopLoop:
	for {
		select {
		case <-waitStopTimeout:
			assert.Fail(t, "timeout waiting for aggregator to be stopped")
		case <-waitStopTick:
			stoppedMu.Lock()
			startExited := expectStartExisted
			stoppedMu.Unlock()
			if startExited {
				break stopLoop
			}
		}
	}
}

func waitForFlowsToBeFlushed(aggregator *FlowAggregator, timeoutDuration time.Duration, minEvents uint64) error {
	timeout := time.After(timeoutDuration)
	tick := time.Tick(500 * time.Millisecond)
	// Keep trying until we're timed out or got a result or got an error
	for {
		select {
		// Got a timeout! fail with a timeout error
		case <-timeout:
			return fmt.Errorf("timeout error waiting for events")
		// Got a tick, we should check on doSomething()
		case <-tick:
			if atomic.LoadUint64(&aggregator.flushedFlowCount) >= minEvents {
				return nil
			}
		}
	}
}