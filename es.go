package restful

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

var gEsUrl = "http://127.0.0.1:9200"
var gEsIndex = "restful"
var gEsIndexAnalyzer = "ik_max_word"
var gEsIndexSearchAnalyzer = "ik_max_word"

var gEsIndexConfigFmt = `{
    "mappings":{
        "_doc":{
		    "properties":{
			    "biz":{
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

func InitEsParam(url, index, analyzer, searchAnalyzer string) error {
	if url != "" {
		gEsUrl = url
		gEsUrl = strings.TrimSuffix(gEsUrl, "/")
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
	return EsEnsureIndex(indexCfg)
}

func EsEnsureIndex(indexCfg string) error {
	url := fmt.Sprintf("%s/%s?include_type_name=true", gEsUrl, gEsIndex)
	statusCode, _, err := HttpDo(url, "", "GET", nil, nil)
	if err != nil {
		return fmt.Errorf("ensure es index get err: %v", err)
	}
	if statusCode == http.StatusNotFound {
		statusCode, indexPutRspData, err := HttpDo(url, "", "PUT", map[string]string{"Content-Type": "application/json; charset=utf-8"}, []byte(indexCfg))
		if err != nil {
			return fmt.Errorf("ensure es index http err: %v", err)
		}
		if statusCode != http.StatusOK && statusCode != http.StatusCreated {
			return fmt.Errorf("ensure es index err: %s", string(indexPutRspData))
		}
	}
	return nil
}

type SearchResponse struct {
	Result string `json:"result"`
	Error  struct {
		Type   string `json:"type"`
		Reason string `json:"reason"`
	} `json:"error"`
	Hits struct {
		Total interface{} `json:"total"`
		Hits  []struct {
			Id     string `json:"_id"`
			Source struct {
				Biz     string `json:"biz"`
				Content string `json:"content"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

func EsUpsert(biz, id, content string) error {
	req := map[string]interface{}{
		"content": content,
		"biz":     biz,
	}
	reqData, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/%s/_doc/%s", gEsUrl, gEsIndex, biz+"_"+id)
	statusCode, rspData, err := HttpDo(url, "", "PUT", map[string]string{"Content-Type": "application/json; charset=utf-8"}, reqData)
	if err != nil {
		return err
	}
	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		rsp := SearchResponse{}
		err = json.Unmarshal(rspData, &rsp)
		if err != nil {
			return err
		}
		return errors.New(fmt.Sprintf("EsUpsert error %v", rsp.Error.Reason))
	}
	return nil
}

func EsRemove(biz, id string) error {
	url := fmt.Sprintf("%s/%s/_doc/%s", gEsUrl, gEsIndex, biz+"_"+id)
	statusCode, rspData, err := HttpDo(url, "", "DELETE", nil, nil)
	if statusCode != http.StatusOK && statusCode != http.StatusNotFound {
		rsp := SearchResponse{}
		err = json.Unmarshal(rspData, &rsp)
		if err != nil {
			return err
		}
		return errors.New(fmt.Sprintf("EsUpsert error %v", rsp.Error.Reason))
	}
	return nil
}

func EsSearch(biz, search string, size, offset int) ([]string, error) {
	req := map[string]interface{}{
		"track_scores": true,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": map[string]interface{}{
					"term": map[string]interface{}{"biz": biz},
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
	url := fmt.Sprintf("%s/%s/_search", gEsUrl, gEsIndex)
	statusCode, rspData, err := HttpDo(url, "", "GET", map[string]string{"Content-Type": "application/json; charset=utf-8"}, reqData)
	if err != nil {
		return nil, err
	}

	var rsp SearchResponse
	err = json.Unmarshal(rspData, &rsp)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("EsSearch error %v", rsp.Error.Reason))
	}

	r := make([]string, 0, len(rsp.Hits.Hits))
	for i := range rsp.Hits.Hits {
		r = append(r, strings.TrimPrefix(rsp.Hits.Hits[i].Id, rsp.Hits.Hits[i].Source.Biz+"_"))
	}
	return r, nil
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

func HttpDo(url, host, method string, header map[string]string, body []byte) (int, []byte, error) {
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
