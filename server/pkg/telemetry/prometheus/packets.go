package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

type Direction string

const (
	Incoming Direction = "incoming"
	Outgoing Direction = "outgoing"
)

var (
	bytesIn    atomic.Uint64
	bytesOut   atomic.Uint64
	packetsIn  atomic.Uint64
	packetsOut atomic.Uint64
	nackTotal  atomic.Uint64

	promPacketLabels = []string{"direction"}

	promPacketTotal *prometheus.CounterVec
	promPacketBytes *prometheus.CounterVec
	promNackTotal   *prometheus.CounterVec
	promPliTotal    *prometheus.CounterVec
	promFirTotal    *prometheus.CounterVec
)

func initPacketStats(nodeID string) {
	promPacketTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   livekitNamespace,
		Subsystem:   "packet",
		Name:        "total",
		ConstLabels: prometheus.Labels{"node_id": nodeID},
	}, promPacketLabels)
	promPacketBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   livekitNamespace,
		Subsystem:   "packet",
		Name:        "bytes",
		ConstLabels: prometheus.Labels{"node_id": nodeID},
	}, promPacketLabels)
	promNackTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   livekitNamespace,
		Subsystem:   "nack",
		Name:        "total",
		ConstLabels: prometheus.Labels{"node_id": nodeID},
	}, promPacketLabels)
	promPliTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   livekitNamespace,
		Subsystem:   "pli",
		Name:        "total",
		ConstLabels: prometheus.Labels{"node_id": nodeID},
	}, promPacketLabels)
	promFirTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   livekitNamespace,
		Subsystem:   "fir",
		Name:        "total",
		ConstLabels: prometheus.Labels{"node_id": nodeID},
	}, promPacketLabels)

	prometheus.MustRegister(promPacketTotal)
	prometheus.MustRegister(promPacketBytes)
	prometheus.MustRegister(promNackTotal)
	prometheus.MustRegister(promPliTotal)
	prometheus.MustRegister(promFirTotal)
}

func IncrementPackets(direction Direction, count uint64) {
	promPacketTotal.WithLabelValues(string(direction)).Add(float64(count))
	if direction == Incoming {
		packetsIn.Add(count)
	} else {
		packetsOut.Add(count)
	}
}

func IncrementBytes(direction Direction, count uint64) {
	promPacketBytes.WithLabelValues(string(direction)).Add(float64(count))
	if direction == Incoming {
		bytesIn.Add(count)
	} else {
		bytesOut.Add(count)
	}
}

func IncrementRTCP(direction Direction, nack, pli, fir uint32) {
	if nack > 0 {
		promNackTotal.WithLabelValues(string(direction)).Add(float64(nack))
		nackTotal.Add(uint64(nack))
	}
	if pli > 0 {
		promPliTotal.WithLabelValues(string(direction)).Add(float64(pli))
	}
	if fir > 0 {
		promFirTotal.WithLabelValues(string(direction)).Add(float64(fir))
	}
}
