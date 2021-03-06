package lib

import (
	"fmt"
)

func (e *Exporter) collectV1(stats Stats, exposedHttpStatusCodes []string, databases []string) error {
	for name, nodeStats := range stats.StatsByNodeName {
		//fmt.Printf("%s -> %v\n", name, stats)
		//glog.Info(fmt.Sprintf("name: %s -> stats: %v\n", name, stats))
		e.authCacheHits.WithLabelValues(name).Set(nodeStats.Couchdb.AuthCacheHits.Current)
		e.authCacheMisses.WithLabelValues(name).Set(nodeStats.Couchdb.AuthCacheMisses.Current)
		e.databaseReads.WithLabelValues(name).Set(nodeStats.Couchdb.DatabaseReads.Current)
		e.databaseWrites.WithLabelValues(name).Set(nodeStats.Couchdb.DatabaseWrites.Current)
		e.openDatabases.WithLabelValues(name).Set(nodeStats.Couchdb.OpenDatabases.Current)
		e.openOsFiles.WithLabelValues(name).Set(nodeStats.Couchdb.OpenOsFiles.Current)
		e.requestTime.WithLabelValues(name).Set(nodeStats.Couchdb.RequestTime.Current)

		for _, code := range exposedHttpStatusCodes {
			if _, ok := nodeStats.HttpdStatusCodes[code]; ok {
				e.httpdStatusCodes.WithLabelValues(code, name).Set(nodeStats.HttpdStatusCodes[code].Current)
			}
		}

		e.httpdRequestMethods.WithLabelValues("COPY", name).Set(nodeStats.HttpdRequestMethods.COPY.Current)
		e.httpdRequestMethods.WithLabelValues("DELETE", name).Set(nodeStats.HttpdRequestMethods.DELETE.Current)
		e.httpdRequestMethods.WithLabelValues("GET", name).Set(nodeStats.HttpdRequestMethods.GET.Current)
		e.httpdRequestMethods.WithLabelValues("HEAD", name).Set(nodeStats.HttpdRequestMethods.HEAD.Current)
		e.httpdRequestMethods.WithLabelValues("POST", name).Set(nodeStats.HttpdRequestMethods.POST.Current)
		e.httpdRequestMethods.WithLabelValues("PUT", name).Set(nodeStats.HttpdRequestMethods.PUT.Current)

		e.bulkRequests.WithLabelValues(name).Set(nodeStats.Httpd.BulkRequests.Current)
		e.clientsRequestingChanges.WithLabelValues(name).Set(nodeStats.Httpd.ClientsRequestingChanges.Current)
		e.requests.WithLabelValues(name).Set(nodeStats.Httpd.Requests.Current)
		e.temporaryViewReads.WithLabelValues(name).Set(nodeStats.Httpd.TemporaryViewReads.Current)
		e.viewReads.WithLabelValues(name).Set(nodeStats.Httpd.ViewReads.Current)
	}

	for name, dbStats := range stats.DatabaseStatsByNodeName {
		for _, dbName := range databases {
			e.diskSize.WithLabelValues(name, dbName).Set(dbStats[dbName].DiskSize)
			e.dataSize.WithLabelValues(name, dbName).Set(dbStats[dbName].DataSize)
			e.diskSizeOverhead.WithLabelValues(name, dbName).Set(dbStats[dbName].DiskSizeOverhead)
		}
	}

	activeTasksByNode := make(map[string]ActiveTaskTypes)
	for _, task := range stats.ActiveTasksResponse {
		if _, ok := activeTasksByNode[task.Node]; !ok {
			activeTasksByNode[task.Node] = ActiveTaskTypes{}
		}
		types := activeTasksByNode[task.Node]

		switch taskType := task.Type; taskType {
		case "database_compaction":
			types.DatabaseCompaction++
			types.Sum++
		case "view_compaction":
			types.ViewCompaction++
			types.Sum++
		case "indexer":
			types.Indexer++
			types.Sum++
		case "replication":
			types.Replication++
			types.Sum++
		default:
			fmt.Printf("unknown task type %s.", taskType)
			types.Sum++
		}
		activeTasksByNode[task.Node] = types
	}
	for nodeName, tasks := range activeTasksByNode {
		e.activeTasks.WithLabelValues(nodeName).Set(tasks.Sum)
		e.activeTasksDatabaseCompaction.WithLabelValues(nodeName).Set(tasks.DatabaseCompaction)
		e.activeTasksViewCompaction.WithLabelValues(nodeName).Set(tasks.ViewCompaction)
		e.activeTasksIndexer.WithLabelValues(nodeName).Set(tasks.Indexer)
		e.activeTasksReplication.WithLabelValues(nodeName).Set(tasks.Replication)
	}

	return nil
}
