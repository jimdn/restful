package restful

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

var gEsURL = "http://127.0.0.1:9200"
var gEsUser = ""
var gEsPwd = ""
var gEsIndex = "restful"
var gEsIndexAnalyzer = "ik_max_word"
var gEsIndexSearchAnalyzer = "ik_max_word"

var gEsIndexConfigFmt = `{
    "mappings":{
        "_doc":{
            "properties":{
                "db":{
                    "type": "keyword"
                },
                "table":{
                    "type": "keyword"
                },
                "content":{
                    "type": "text",
                    "analyzer": "%s",
                    "search_analyzer": "%s"
                }
            }
        }
    },
    "settings":{
        "index":{
            "number_of_shards" : 1,
            "number_of_replicas" : 2,
            "max_result_window": 1000000
        }
    }
}`

func initEsParam(url, user, pwd, index, analyzer, searchAnalyzer string) error {
	if url != "" {
		gEsURL = url
		gEsURL = strings.TrimSuffix(gEsURL, "/")
	}
	if user != "" {
		gEsUser = user
	}
	if pwd != "" {
		gEsPwd = pwd
	}
	if index != "" {
		gEsIndex = index
	}
	if analyzer != "" {
		gEsIndexAnalyzer = analyzer
	}
	if searchAnalyzer != "" {
		gEsIndexSearchAnalyzer = searchAnalyzer
	}
	indexCfg := fmt.Sprintf(gEsIndexConfigFmt, gEsIndexAnalyzer, gEsIndexSearchAnalyzer)
	return esEnsureIndex(indexCfg)
}

func esEnsureIndex(indexCfg string) error {
	url := fmt.Sprintf("%s/%s?include_type_name=true", gEsURL, gEsIndex)
	header := make(map[string]string)
	header["Content-Type"] = "application/json; charset=utf-8"
	if gEsUser != "" || gEsPwd != "" {
		header["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(gEsUser+":"+gEsPwd))
	}
	statusCode, _, err := httpDo(url, "", "GET", header, nil)
	if err != nil {
		return fmt.Errorf("ensure es index get err: %v", err)
	}
	if statusCode == http.StatusNotFound {
		statusCode, indexPutRspData, err := httpDo(url, "", "PUT", header, []byte(indexCfg))
		if err != nil {
			return fmt.Errorf("ensure es index http err: %v", err)
		}
		if statusCode != http.StatusOK && statusCode != http.StatusCreated {
			return fmt.Errorf("ensure es index err: %s", string(indexPutRspData))
		}
	}
	return nil
}

// SearchResponse is the rsp structure of es
type SearchResponse struct {
	Result string `json:"result"`
	Error  struct {
		Type   string `json:"type"`
		Reason string `json:"reason"`
	} `json:"error"`
	Hits struct {
		Total int64 `json:"total"`
		Hits  []struct {
			ID     string `json:"_id"`
			Source struct {
				Db      string `json:"db"`
				Table   string `json:"table"`
				Content string `json:"content"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

func esUpsert(db, table, id, content string) error {
	req := map[string]interface{}{
		"db":      db,
		"table":   table,
		"content": content,
	}
	reqData, _ := json.Marshal(req)
	docID := fmt.Sprintf("%s_%s_%s", db, table, id)
	destURL := fmt.Sprintf("%s/%s/_doc/%s", gEsURL, gEsIndex, docID)
	header := make(map[string]string)
	header["Content-Type"] = "application/json; charset=utf-8"
	if gEsUser != "" || gEsPwd != "" {
		header["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(gEsUser+":"+gEsPwd))
	}
	statusCode, rspData, err := httpDo(destURL, "", "PUT", header, reqData)
	if err != nil {
		return err
	}
	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		rsp := SearchResponse{}
		err = json.Unmarshal(rspData, &rsp)
		if err != nil {
			return err
		}
		return fmt.Errorf("EsUpsert error %v", rsp.Error.Reason)
	}
	return nil
}

func esRemove(db, table, id string) error {
	docID := fmt.Sprintf("%s_%s_%s", db, table, id)
	destURL := fmt.Sprintf("%s/%s/_doc/%s", gEsURL, gEsIndex, docID)
	header := make(map[string]string)
	header["Content-Type"] = "application/json; charset=utf-8"
	if gEsUser != "" || gEsPwd != "" {
		header["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(gEsUser+":"+gEsPwd))
	}
	statusCode, rspData, err := httpDo(destURL, "", "DELETE", header, nil)
	if err != nil {
		return err
	}
	if statusCode != http.StatusOK && statusCode != http.StatusNotFound {
		rsp := SearchResponse{}
		err = json.Unmarshal(rspData, &rsp)
		if err != nil {
			return err
		}
		return fmt.Errorf("EsUpsert error %v", rsp.Error.Reason)
	}
	return nil
}

func esSearch(db, table, search string, size, offset int) ([]string, error) {
	req := map[string]interface{}{
		"track_scores": true,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{"db": db}},
					{"term": map[string]interface{}{"table": table}},
				},
				"must": map[string]interface{}{
					"match": map[string]interface{}{
						"content": map[string]interface{}{
							"query":    search,
							"operator": "and",
						},
					},
				},
			},
		},
		"size": size,
		"from": offset,
	}

	reqData, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/%s/_search?rest_total_hits_as_int=true", gEsURL, gEsIndex)
	header := make(map[string]string)
	header["Content-Type"] = "application/json; charset=utf-8"
	if gEsUser != "" || gEsPwd != "" {
		header["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(gEsUser+":"+gEsPwd))
	}
	statusCode, rspData, err := httpDo(url, "", "GET", header, reqData)
	if err != nil {
		return nil, err
	}

	var rsp SearchResponse
	err = json.Unmarshal(rspData, &rsp)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("EsSearch error %v", rsp.Error.Reason)
	}

	docIDs := make([]string, 0, len(rsp.Hits.Hits))
	for i := range rsp.Hits.Hits {
		idPrefix := fmt.Sprintf("%s_%s_", db, table)
		docIDs = append(docIDs, strings.TrimPrefix(rsp.Hits.Hits[i].ID, idPrefix))
	}
	return docIDs, nil
}

var gNetClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:          2000,
		MaxIdleConnsPerHost:   100,
		ResponseHeaderTimeout: 3 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
	Timeout: 4 * time.Second,
}

func httpDo(url, host, method string, header map[string]string, body []byte) (int, []byte, error) {
	var err error
	var req *http.Request
	if body != nil {
		reqBody := bytes.NewBuffer(body)
		req, err = http.NewRequest(method, url, reqBody)
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return 0, nil, err
	}
	if len(host) != 0 {
		req.Host = host
	}
	if header != nil {
		for k, v := range header {
			req.Header.Set(k, v)
		}
	}

	var rsp *http.Response
	rsp, err = gNetClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer rsp.Body.Close()

	rspBody, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return 0, nil, err
	}
	return rsp.StatusCode, rspBody, nil
}
