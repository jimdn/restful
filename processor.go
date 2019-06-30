package restful

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)


type Processor struct {
	// Business name, usually using plural noun, the uses are as follows:
	//   1. default db name: rest_{Biz}
	//   2. value of elasticsearch biz field
	//   3. logs
	Biz string

	// URL Path as service, usually equal to Biz
	URLPath string

	// for fields type parsing
	DataStruct interface{}

	// fields for search
	// to use the search feature, you must enable GlobalConfig.EsEnable
	// field's type must be string or []string
	SearchFields []string

	// fields CreateOnly
	// fields can only be written when creating by POST or PUT
	CreateOnlyFields []string

	// fields ReadOnly
	// fields can not be written or update, data should be loaded into DB by other ways
	ReadOnlyFields []string

	// fields type and R/W config
	FieldSet *FieldSet

	// CURD handler
	PostHandler Handler
	PutHandler Handler
	PatchHandler Handler
	GetHandler Handler
	GetPageHandler Handler
	DeleteHandler Handler

	// Do something after data write success
	//   1. update search data to es
	OnWriteDone func(method string, vars map[string]string, query url.Values, data map[string]interface{})

	// specify db and collection name from URL Query
	// e.g.: /path?db=dbName&col=colName
	// default db name: rest_{Biz}
	// default col name: cn
	GetDbName func(query url.Values) string
	GetColName func(query url.Values) string
}


func (p *Processor) Init() error {
	p.FieldSet = BuildFieldSet(reflect.TypeOf(p.DataStruct))

	// DataStruct must contain 'id', 'btime', 'mtime', 'seq' fields
	//   id: primary key
	//   btime: means birth time, the time when the doc created
	//   mtime: means modify time, the time when the doc modified
	//   seq: means the version of the doc
	if _, ok := p.FieldSet.FMap["id"]; !ok {
		return fmt.Errorf("%s struct must contain 'id' field", p.Biz)
	}
	if _, ok := p.FieldSet.FMap["btime"]; !ok {
		return fmt.Errorf("%s struct must contain 'btime' field", p.Biz)
	}
	if _, ok := p.FieldSet.FMap["mtime"]; !ok {
		return fmt.Errorf("%s struct must contain 'mtime' field", p.Biz)
	}
	if _, ok := p.FieldSet.FMap["seq"]; !ok {
		return fmt.Errorf("%s struct must contain 'seq' field", p.Biz)
	}

	err := p.FieldSet.CheckSearchFields(p.SearchFields)
	if err != nil {
		return fmt.Errorf("%s %s", p.Biz, err.Error())
	}

	p.FieldSet.SetCreateOnlyFields(p.CreateOnlyFields)
	p.FieldSet.SetReadOnlyFields(p.ReadOnlyFields)

	Log.Debugf("%v FieldSet %v", p.Biz, p.FieldSet)

	// default value
	p.GetDbName = p.DefaultGetDbName()
	p.GetColName = p.DefaultGetColName()
	p.PostHandler = p.DefaultPost()
	p.PutHandler = p.DefaultPut()
	p.PatchHandler = p.DefaultPatch()
	p.GetHandler = p.DefaultGet()
	p.GetPageHandler = p.DefaultGetPage()
	p.DeleteHandler = p.DefaultDelete()
	p.OnWriteDone = p.DefaultOnWriteDone()

	return nil
}

func (p *Processor) Load() {
	path := p.URLPath
	pathWithId := p.URLPath + "/{id}"
	Register("POST",   path,       p.PostHandler)
	Register("PUT",    pathWithId, p.PutHandler)
	Register("PATCH",  pathWithId, p.PatchHandler)
	Register("GET",    pathWithId, p.GetHandler)
	Register("GET",    path,       p.GetPageHandler)
	Register("DELETE", pathWithId, p.DeleteHandler)
}


func (p *Processor) DefaultGetDbName() func(query url.Values) string {
	return func(query url.Values) string {
		if db := query.Get("db"); db != "" {
			return db
		}
		return "rest_" + p.Biz
	}
}

func (p *Processor) DefaultGetColName() func(query url.Values) string {
	return func(query url.Values) string {
		if col := query.Get("col"); col != "" {
			return col
		}
		return "cn"
	}
}

func (p *Processor) DefaultPost() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		var err error
		var info map[string]interface{}
		if err = json.Unmarshal(body, &info); err != nil {
			Log.Warnf("unmarshal fail %v [%v]", err, string(body))
			return genRsp(http.StatusBadRequest, "invalid Body", nil)
		}

		if id, ok := info["id"]; ok {
			v := GetString(id)
			if len(v) > 64 || v == "" {
				Log.Warnf("custom id too long or empty")
				return genRsp(http.StatusBadRequest, "custom id too long or empty", nil)
			}
		} else {
			info["id"] = UUID()
		}

		err = p.FieldSet.CheckObject(info, false)
		if err != nil {
			Log.Warnf("invalid field exists, biz=%v err=%v", p.Biz, err)
			return genRsp(http.StatusBadRequest, err.Error(), nil)
		}
		p.FieldSet.InReplace(&info)

		now := time.Now().Unix()
		info["btime"] = now
		info["mtime"] = now
		info["seq"] = GenSeq(0)

		dbs := gCfg.MgoSess.Clone()
		defer dbs.Close()
		dbc := dbs.DB(p.GetDbName(query)).C(p.GetColName(query))

		doc := p.FieldSet.InSort(&info)
		err = dbc.Insert(&doc)
		if err != nil {
			Log.Warnf("db access fail, err=%v", err)
			if mgo.IsDup(err) {
				return genRsp(http.StatusBadRequest, "duplicate id", nil)
			}
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}

		if p.OnWriteDone != nil {
			go p.OnWriteDone("POST", vars, query, info)
		}
		return genRsp(http.StatusOK, "post ok", map[string]interface{}{"id": info["_id"]})
	}
}

func (p *Processor) DefaultPut() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		var err error
		var info map[string]interface{}
		if err = json.Unmarshal(body, &info); err != nil {
			Log.Warnf("unmarshal fail %v [%v]", err, string(body))
			return genRsp(http.StatusBadRequest, "invalid Body", nil)
		}

		id := vars["id"]
		info["id"] = id
		err = p.FieldSet.CheckObject(info, false)
		if err != nil {
			Log.Warnf("invalid field exists, biz=%v err=%v", p.Biz, err)
			return genRsp(http.StatusBadRequest, err.Error(), nil)
		}
		p.FieldSet.InReplace(&info)

		now := time.Now().Unix()
		info["btime"] = now
		info["mtime"] = now
		info["seq"] = GenSeq(0)

		dbs := gCfg.MgoSess.Clone()
		defer dbs.Close()
		dbc := dbs.DB(p.GetDbName(query)).C(p.GetColName(query))

		var old map[string]interface{}
		err = dbc.Find(bson.M{"_id": id}).Select(bson.M{"btime": 1, "seq": 1}).One(&old)
		if err == nil {
			if v, ok := old["btime"]; ok {
				info["btime"] = v
			} else {
				info["btime"] = now
			}

			if v, ok := old["seq"]; ok {
				nextSeq, err2 := NextSeq(v.(string))
				if err2 == nil {
					info["seq"] = nextSeq
				} else {
					info["seq"] = GenSeq(0)
				}
			}
		} else if err != mgo.ErrNotFound {
			Log.Warnf("db access fail, err=%v", err)
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}

		doc := p.FieldSet.InSort(&info)
		_, err = dbc.Upsert(bson.M{"_id": id}, &doc)
		if err != nil {
			Log.Warnf("db access fail, err=%v", err)
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}

		if p.OnWriteDone != nil {
			go p.OnWriteDone("PUT", vars, query, info)
		}
		return genRsp(http.StatusOK, "put ok", map[string]interface{}{"id": info["_id"]})
	}
}

func (p *Processor) DefaultPatch() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		var err error
		var info map[string]interface{}
		if err = json.Unmarshal(body, &info); err != nil {
			Log.Warnf("unmarshal fail %v [%v]", err, string(body))
			return genRsp(http.StatusBadRequest, "invalid Body", nil)
		}

		err = p.FieldSet.CheckObject(info, true)
		if err != nil {
			Log.Warnf("invalid field exists, biz=%v err=%v", p.Biz, err)
			return genRsp(http.StatusBadRequest, err.Error(), nil)
		}
		p.FieldSet.InReplace(&info)

		// check seq param
		seq := query.Get("seq")
		ignoreSeq := false
		if strings.ToLower(query.Get("ignore_seq")) == "true" {
			ignoreSeq = true
		}
		if !ignoreSeq && seq == "" {
			Log.Warnf("need seq")
			return genRsp(http.StatusBadRequest, "need seq", nil)
		}

		dbs := gCfg.MgoSess.Clone()
		defer dbs.Close()
		dbc := dbs.DB(p.GetDbName(query)).C(p.GetColName(query))

		id := vars["id"]
		now := time.Now().Unix()
		if ignoreSeq {
			if _, ok := info["seq"]; ok {
				delete(info, "seq")
			}
			info["mtime"] = now
			err = dbc.Update(bson.M{"_id": id}, bson.M{"$set": info})
		} else {
			nextSeq, err2 := NextSeq(seq)
			if err2 != nil {
				Log.Warnf("invalid seq: %s", seq)
				return genRsp(http.StatusBadRequest, "invalid seq", nil)
			}
			info["seq"] = nextSeq
			info["mtime"] = now
			err = dbc.Update(bson.M{"_id": id, "seq": seq}, bson.M{"$set": info})
			if err == mgo.ErrNotFound {
				Log.Warnf("id not found or seq conflict")
				return genRsp(http.StatusBadRequest, "id not found or seq conflict", nil)
			}
		}

		if err != nil {
			Log.Warnf("db access fail, err=%v", err)
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}

		if p.OnWriteDone != nil {
			go p.OnWriteDone("PATCH", vars, query, info)
		}
		return genRsp(http.StatusOK, "patch ok", map[string]interface{}{"id": id})
	}
}

func (p *Processor) DefaultGet() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		var err error
		id := vars["id"]

		// build select
		selector := make(map[string]interface{})
		if query.Get("select") != "" {
			var selSlice []string
			err := json.Unmarshal([]byte(query.Get("select")), &selSlice)
			if err != nil {
				Log.Warnf("unmarshal select error: %v", err)
				return genRsp(http.StatusBadRequest, "select invalid", nil)
			}
			err = p.FieldSet.BuildSelectObj(selSlice, selector)
			if err != nil {
				Log.Warnf("select param invalid, %v", err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		p.FieldSet.InReplace(&selector)

		dbs := gCfg.MgoSess.Clone()
		defer dbs.Close()
		dbc := dbs.DB(p.GetDbName(query)).C(p.GetColName(query))

		var info map[string]interface{}
		err = dbc.Find(bson.M{"_id": id}).Select(selector).One(&info)
		if err != nil {
			Log.Warnf("get id=%s error, %v", id, err)
			if err == mgo.ErrNotFound {
				return genRsp(http.StatusNotFound, "id not found", nil)
			}
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}
		p.FieldSet.OutReplace(&info)
		return genRsp(http.StatusOK, "get ok", info)
	}
}

func (p *Processor) DefaultGetPage() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		var err error
		size := 0
		page := 0

		size, err = strconv.Atoi(query.Get("size"))
		if err != nil || (size <= 0 && size != -1) {
			Log.Warnf("size error")
			return genRsp(http.StatusBadRequest, "need size or size invalid", nil)
		}

		page, err = strconv.Atoi(query.Get("page"))
		if err != nil || page <= 0 {
			Log.Warnf("page error")
			return genRsp(http.StatusBadRequest, "need page or page invalid", nil)
		}

		// build condition
		condition := make(map[string]interface{})
		if query.Get("filter") != "" {
			var filter map[string]interface{}
			err := json.Unmarshal([]byte(query.Get("filter")), &filter)
			if err != nil {
				Log.Warnf("unmarshal filter error: %v", err)
				return genRsp(http.StatusBadRequest, "filter invalid", nil)
			}
			err = p.FieldSet.BuildFilterObj(filter, condition)
			if err != nil {
				Log.Warnf("filter param invalid, %v", err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		if query.Get("range") != "" {
			var rang map[string]interface{}
			err := json.Unmarshal([]byte(query.Get("range")), &rang)
			if err != nil {
				Log.Warnf("unmarshal range error: %v", err)
				return genRsp(http.StatusBadRequest, "range invalid", nil)
			}
			err = p.FieldSet.BuildRangeObj(rang, condition)
			if err != nil {
				Log.Warnf("range param invalid, %v", err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		if query.Get("in") != "" {
			var in map[string]interface{}
			err := json.Unmarshal([]byte(query.Get("in")), &in)
			if err != nil {
				Log.Warnf("unmarshal in error: %v", err)
				return genRsp(http.StatusBadRequest, "in invalid", nil)
			}
			err = p.FieldSet.BuildInObj(in, condition)
			if err != nil {
				Log.Warnf("in param invalid, %v", err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		if query.Get("nin") != "" {
			var nin map[string]interface{}
			err := json.Unmarshal([]byte(query.Get("nin")), &nin)
			if err != nil {
				Log.Warnf("unmarshal nin error: %v", err)
				return genRsp(http.StatusBadRequest, "nin invalid", nil)
			}
			err = p.FieldSet.BuildNinObj(nin, condition)
			if err != nil {
				Log.Warnf("nin param invalid, %v", err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		if query.Get("all") != "" {
			var all map[string]interface{}
			err := json.Unmarshal([]byte(query.Get("all")), &all)
			if err != nil {
				Log.Warnf("unmarshal all error: %v", err)
				return genRsp(http.StatusBadRequest, "all invalid", nil)
			}
			err = p.FieldSet.BuildAllObj(all, condition)
			if err != nil {
				Log.Warnf("all param invalid, %v", err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		if query.Get("search") != "" {
			search := query.Get("search")
			if search != "" {
				if !gCfg.EsEnable {
					Log.Warnf("search not config")
					return genRsp(http.StatusInternalServerError, "search not config", nil)
				}
				ids, err := EsSearch(p.Biz, search, 2000, 0)
				if err != nil {
					Log.Warnf("EsSearch err, %v", err)
					return genRsp(http.StatusInternalServerError, err.Error(), nil)
				}
				if len(ids) == 0 {
					infos := make([]interface{}, 0)
					return genRsp(http.StatusOK, "no results found", RspGetPageData{Total: 0, Hits: infos})
				} else {
					if _, exist := condition["id"]; exist {
						Log.Warnf("search id condition conflict")
						return genRsp(http.StatusBadRequest, "search id condition conflict", nil)
					}
					condition["id"] = map[string]interface{}{"$in": ids}
				}
			}
		}
		p.FieldSet.InReplace(&condition)

		// build sort
		sort := make(bson.D, 0, 0)
		if query.Get("order") != "" {
			var order []string
			err := json.Unmarshal([]byte(query.Get("order")), &order)
			if err != nil {
				Log.Warnf("unmarshal order error: %v", err)
				return genRsp(http.StatusBadRequest, "order invalid", nil)
			}
			err = p.FieldSet.BuildOrderArray(order, &sort)
			if err != nil {
				Log.Warnf("order param invalid, %v", err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		orderFields := p.FieldSet.OrderArray2Slice(&sort)

		// build select
		selector := make(map[string]interface{})
		if query.Get("select") != "" {
			var selSlice []string
			err := json.Unmarshal([]byte(query.Get("select")), &selSlice)
			if err != nil {
				Log.Warnf("unmarshal select error: %v", err)
				return genRsp(http.StatusBadRequest, "select invalid", nil)
			}
			err = p.FieldSet.BuildSelectObj(selSlice, selector)
			if err != nil {
				Log.Warnf("select param invalid, %v", err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		p.FieldSet.InReplace(&selector)

		Log.Debugf("query=%v, condition=%v, order=%v, select=%v", query, condition, orderFields, selector)

		dbs := gCfg.MgoSess.Clone()
		defer dbs.Close()
		dbc := dbs.DB(p.GetDbName(query)).C(p.GetColName(query))

		// count
		total := 0
		total, err = dbc.Find(condition).Count()
		if err != nil {
			Log.Warnf("get page count error: %v", err)
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}
		if total <= 0 {
			infos := make([]interface{}, 0)
			return genRsp(http.StatusOK, "no results found", RspGetPageData{Total: 0, Hits: infos})
		}

		// results
		var infos []interface{}
		switch {
		case size == -1:
			err = dbc.Find(condition).Sort(orderFields...).Select(selector).All(&infos)
		case size > 0:
			err = dbc.Find(condition).Skip(size * (page-1)).Limit(size).Sort(orderFields...).Select(selector).All(&infos)
		default:
			err = fmt.Errorf("unknown")
		}
		if err != nil {
			Log.Warnf("get page results error: %v", err)
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}

		p.FieldSet.OutReplaceArray(infos)
		return genRsp(http.StatusOK, "get page ok", RspGetPageData{Total: int64(total), Hits: infos})
	}
}

func (p *Processor) DefaultDelete() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		var err error
		id := vars["id"]

		dbs := gCfg.MgoSess.Clone()
		defer dbs.Close()
		dbc := dbs.DB(p.GetDbName(query)).C(p.GetColName(query))

		err = dbc.Remove(bson.M{"_id": id})
		if err != nil {
			Log.Warnf("delete id=%s error, %v", id, err)
			if err == mgo.ErrNotFound {
				return genRsp(http.StatusNotFound, "id not found", nil)
			}
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}

		if p.OnWriteDone != nil {
			go p.OnWriteDone("DELETE", vars, query, nil)
		}
		return genRsp(http.StatusOK, "delete ok", map[string]interface{}{"id": id})
	}
}

func (p *Processor) DefaultOnWriteDone() func(method string, vars map[string]string, query url.Values, data map[string]interface{}) {
	return func(method string, vars map[string]string, query url.Values, data map[string]interface{}) {
		var err error
		switch method {
		case "POST":
			fallthrough
		case "PUT":
			if gCfg.EsEnable {
				id := GetString(data["_id"])
				content := p.FieldSet.BuildSearchContent(data, p.SearchFields)
				if content != "" {
					err = EsUpsert(p.Biz, id, content)
				} else {
					err = EsRemove(p.Biz, id)
				}
			}
		case "PATCH":
			if gCfg.EsEnable {
				dbs := gCfg.MgoSess.Clone()
				defer dbs.Close()
				dbc := dbs.DB(p.GetDbName(query)).C(p.GetColName(query))
				id := vars["id"]
				var info map[string]interface{}
				err = dbc.Find(bson.M{"_id": id}).One(&info)
				if err != nil {
					Log.Warnf("OnWriteDone [%v][%v] db fail %v", p.Biz, method, err)
					return
				}
				content := p.FieldSet.BuildSearchContent(info, p.SearchFields)
				if content != "" {
					err = EsUpsert(p.Biz, id, content)
				} else {
					err = EsRemove(p.Biz, id)
				}
			}
		case "DELETE":
			if gCfg.EsEnable {
				id := vars["id"]
				err = EsRemove(p.Biz, id)
			}
		}
		if err != nil {
			Log.Warnf("OnWriteDone [%v][%v] es access fail %v", p.Biz, method, err)
		}
	}
}
