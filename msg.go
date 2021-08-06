package restful

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
)

// Rsp is a general returning structure for all request
type Rsp struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// RspGetPageData is a general returning structure in `data` field for GetPage request
type RspGetPageData struct {
	Total int64         `json:"total"`
	Hits  []interface{} `json:"hits"`
}

// Handler is a template function for Restful Handler
type Handler func(vars map[string]string, query url.Values, body []byte) *Rsp

// Register is a function to register handler to http mux
func Register(method, pattern string, h Handler) {
	handler := genHandler(h)
	gCfg.Mux.HandleFunc(pattern, handler).Methods(method)
}

func genHandler(h Handler) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var rsp *Rsp
		vars := mux.Vars(r)
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			rsp = genRsp(http.StatusBadRequest, fmt.Sprintf("query parser failed: %v", err), nil)
			writeRsp(w, rsp, false)
			return
		}
		pretty := false
		if strings.ToLower(query.Get("pretty")) == "true" {
			pretty = true
		}

		if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				rsp = genRsp(http.StatusInternalServerError, fmt.Sprintf("read body error: %v", err), nil)
				writeRsp(w, rsp, pretty)
				return
			}
			defer r.Body.Close()
			rsp = h(vars, query, body)
		} else {
			rsp = h(vars, query, nil)
		}
		writeRsp(w, rsp, pretty)
	}
}

func genRsp(code int, msg string, data interface{}) *Rsp {
	return &Rsp{
		Code: code,
		Msg:  msg,
		Data: data,
	}
}

func writeRsp(w http.ResponseWriter, rsp *Rsp, pretty bool) {
	statusCode := rsp.Code
	if statusCode >= 100 && statusCode < 400 {
		rsp.Code = 0
	}
	var pBuf *[]byte
	if pretty {
		buf, _ := json.MarshalIndent(rsp, "", "    ")
		pBuf = &buf
	} else {
		buf, _ := json.Marshal(rsp)
		pBuf = &buf
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	w.Write(*pBuf)
}
