package lib

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	exposedHttpStatusCodes = []string{"200", "201", "202", "301", "304", "400", "401", "403", "404", "405", "409", "412", "500"}
)

// Describe describes all the metrics ever exported by the couchdb exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up.Desc()

	e.authCacheHits.Describe(ch)
	e.authCacheMisses.Describe(ch)
	e.databaseReads.Describe(ch)
	e.databaseWrites.Describe(ch)
	e.openDatabases.Describe(ch)
	e.openOsFiles.Describe(ch)
	e.requestTime.Describe(ch)

	e.httpdStatusCodes.Describe(ch)
	e.httpdRequestMethods.Describe(ch)

	e.bulkRequests.Describe(ch)
	e.clientsRequestingChanges.Describe(ch)
	e.requests.Describe(ch)
	e.temporaryViewReads.Describe(ch)
	e.viewReads.Describe(ch)
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {
	sendStatus := func() {
		ch <- e.up
	}
	defer sendStatus()

	e.up.Set(0)
	stats, err := e.client.getStats()
	if err != nil {
		return fmt.Errorf("Error reading couchdb stats: %v", err)
	}

	e.up.Set(1)

	if stats.ApiVersion == "2" {
		e.collectV2(stats, exposedHttpStatusCodes)
	} else {
		e.collectV1(stats, exposedHttpStatusCodes)
	}

	e.authCacheHits.Collect(ch)
	e.authCacheMisses.Collect(ch)
	e.databaseReads.Collect(ch)
	e.databaseWrites.Collect(ch)
	e.openDatabases.Collect(ch)
	e.openOsFiles.Collect(ch)
	e.requestTime.Collect(ch)
	e.httpdStatusCodes.Collect(ch)
	e.httpdRequestMethods.Collect(ch)
	e.bulkRequests.Collect(ch)
	e.clientsRequestingChanges.Collect(ch)
	e.requests.Collect(ch)
	e.temporaryViewReads.Collect(ch)
	e.viewReads.Collect(ch)

	return nil
}

// Collect fetches the stats from configured couchdb location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()
	if err := e.collect(ch); err != nil {
		glog.Error(fmt.Sprintf("Error collecting stats: %s", err))
	}
	return
}
