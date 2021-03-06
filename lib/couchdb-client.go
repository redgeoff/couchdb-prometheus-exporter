package lib

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/hashicorp/go-version"
	"io/ioutil"
	"net/http"
)

var (
	insecure = flag.Bool("insecure", true, "Ignore server certificate if using https")
)

type BasicAuth struct {
	Username string
	Password string
}

type CouchdbClient struct {
	baseUri   string
	basicAuth BasicAuth
	databases []string
	client    *http.Client
}

type RootResponse struct {
	Couchdb string `json:"couchdb"`
	Version string `json:"version"`
}

type MembershipResponse struct {
	AllNodes     []string `json:"all_nodes"`
	ClusterNodes []string `json:"cluster_nodes"`
}

func (c *CouchdbClient) getServerVersion() (string, error) {
	data, err := c.request("GET", fmt.Sprintf("%s/", c.baseUri))
	if err != nil {
		return "", err
	}
	var root RootResponse
	err = json.Unmarshal(data, &root)
	if err != nil {
		return "", err
	}
	return root.Version, nil
}

func (c *CouchdbClient) isCouchDbV2() (bool, error) {
	clusteredCouch, err := version.NewConstraint(">= 2.0")
	if err != nil {
		return false, err
	}

	serverVersion, err := c.getServerVersion()
	if err != nil {
		return false, err
	}

	couchDbVersion, err := version.NewVersion(serverVersion)
	if err != nil {
		return false, err
	}

	glog.Infof("relaxing on couch@%s", couchDbVersion)
	//fmt.Printf("relaxing on couch@%s\n", couchDbVersion)
	return clusteredCouch.Check(couchDbVersion), nil
}

func (c *CouchdbClient) getNodeNames() ([]string, error) {
	data, err := c.request("GET", fmt.Sprintf("%s/_membership", c.baseUri))
	if err != nil {
		return nil, err
	}
	var membership MembershipResponse
	err = json.Unmarshal(data, &membership)
	if err != nil {
		return nil, err
	}
	for i, name := range membership.ClusterNodes {
		glog.Infof("node[%d]: %s\n", i, name)
	}
	return membership.ClusterNodes, nil
}

func (c *CouchdbClient) getNodeBaseUrisByNodeName(baseUri string) (map[string]string, error) {
	names, err := c.getNodeNames()
	if err != nil {
		return nil, err
	}
	urisByNodeName := make(map[string]string)
	for _, name := range names {
		urisByNodeName[name] = fmt.Sprintf("%s/_node/%s", baseUri, name)
	}
	return urisByNodeName, nil
}

func (c *CouchdbClient) getStatsByNodeName(urisByNodeName map[string]string) (map[string]StatsResponse, error) {
	statsByNodeName := make(map[string]StatsResponse)
	for name, uri := range urisByNodeName {
		data, err := c.request("GET", fmt.Sprintf("%s/_stats", uri))
		if err != nil {
			return nil, fmt.Errorf("Error reading couchdb stats: %v", err)
		}

		var stats StatsResponse
		err = json.Unmarshal(data, &stats)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling stats: %v", err)
		}
		statsByNodeName[name] = stats
	}
	return statsByNodeName, nil
}

func (c *CouchdbClient) getStats() (Stats, error) {
	isCouchDbV2, err := c.isCouchDbV2()
	if err != nil {
		return Stats{}, err
	}
	if isCouchDbV2 {
		urisByNode, err := c.getNodeBaseUrisByNodeName(c.baseUri)
		if err != nil {
			return Stats{}, err
		}
		nodeStats, err := c.getStatsByNodeName(urisByNode)
		if err != nil {
			return Stats{}, err
		}
		databaseStats, err := c.getDatabasesStatsByNodeName(urisByNode)
		if err != nil {
			return Stats{}, err
		}
		activeTasks, err := c.getActiveTasks()
		if err != nil {
			return Stats{}, err
		}
		return Stats{
			StatsByNodeName:         nodeStats,
			DatabaseStatsByNodeName: databaseStats,
			ActiveTasksResponse:     activeTasks,
			ApiVersion:              "2"}, nil
	} else {
		urisByNode := map[string]string{
			"master": c.baseUri,
		}
		nodeStats, err := c.getStatsByNodeName(urisByNode)
		if err != nil {
			return Stats{}, err
		}
		databaseStats, err := c.getDatabasesStatsByNodeName(urisByNode)
		if err != nil {
			return Stats{}, err
		}
		activeTasks, err := c.getActiveTasks()
		if err != nil {
			return Stats{}, err
		}
		return Stats{
			StatsByNodeName:         nodeStats,
			DatabaseStatsByNodeName: databaseStats,
			ActiveTasksResponse:     activeTasks,
			ApiVersion:              "1"}, nil
	}
}

func (c *CouchdbClient) getDatabasesStatsByNodeName(urisByNodeName map[string]string) (map[string]DatabaseStatsByDbName, error) {
	dbStatsByDbName := make(map[string]DatabaseStatsByDbName)
	for name, _ := range urisByNodeName {
		dbStatsByDbName[name] = make(map[string]DatabaseStats)
		for _, dbName := range c.databases {
			data, err := c.request("GET", fmt.Sprintf("%s/%s", c.baseUri, dbName))
			if err != nil {
				return nil, fmt.Errorf("Error reading database '%s' stats: %v", dbName, err)
			}

			var dbStats DatabaseStats
			err = json.Unmarshal(data, &dbStats)
			if err != nil {
				return nil, fmt.Errorf("error unmarshalling database '%s' stats: %v", dbName, err)
			}
			dbStats.DiskSizeOverhead = dbStats.DiskSize - dbStats.DataSize
			dbStatsByDbName[name][dbName] = dbStats
		}
	}
	return dbStatsByDbName, nil
}

func (c *CouchdbClient) getActiveTasks() (ActiveTasksResponse, error) {
	data, err := c.request("GET", fmt.Sprintf("%s/_active_tasks", c.baseUri))
	if err != nil {
		return ActiveTasksResponse{}, fmt.Errorf("Error reading active tasks: %v", err)
	}

	var activeTasks ActiveTasksResponse
	err = json.Unmarshal(data, &activeTasks)
	if err != nil {
		return ActiveTasksResponse{}, fmt.Errorf("error unmarshalling active tasks: %v", err)
	}
	for _, activeTask := range activeTasks {
		// CouchDB 1.x doesn't know anything about nodes.
		if activeTask.Node != "" {
			activeTask.Node = "master"
		}
	}
	return activeTasks, nil
}

func (c *CouchdbClient) request(method string, uri string) (respData []byte, err error) {
	req, err := http.NewRequest(method, uri, nil)
	if err != nil {
		return nil, err
	}
	if len(c.basicAuth.Username) > 0 {
		req.SetBasicAuth(c.basicAuth.Username, c.basicAuth.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	respData, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		if err != nil {
			respData = []byte(err.Error())
		}
		return nil, fmt.Errorf("Status %s (%d): %s", resp.Status, resp.StatusCode, respData)
	}

	return respData, nil
}

func NewCouchdbClient(uri string, basicAuth BasicAuth, databases []string) *CouchdbClient {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: *insecure},
		},
	}

	return &CouchdbClient{
		baseUri:   uri,
		basicAuth: basicAuth,
		databases: databases,
		client:    httpClient,
	}
}
