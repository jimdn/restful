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

// Processor
type Processor struct {
	// Business name, functions:
	//   1. default value of TableName
	//   2. default value of URLPath
	//   3. logs
	Biz string

	// Table name, using ${Biz} if empty
	TableName string

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

	// indexes will be created in database/table
	Indexes []Index

	// fields type and R/W config
	FieldSet *FieldSet

	// CURD handler
	PostHandler    Handler
	PutHandler     Handler
	PatchHandler   Handler
	GetHandler     Handler
	GetPageHandler Handler
	DeleteHandler  Handler
	TriggerHandler Handler

	// Do something after data write success
	//   1. update search data to es
	OnWriteDone func(method string, vars map[string]string, query url.Values, data map[string]interface{})

	// specify db and table name from URL Query
	// e.g.: /path?db=dbName&table=tableName
	// default db name: restful
	// default table name: ${TableName}
	GetDbName  func(query url.Values) string
	GetTableName func(query url.Values) string
}

// Init a processor
func (p *Processor) Init() error {
	if p.Biz == "" {
		return fmt.Errorf("biz is empty")
	}
	if p.TableName == "" {
		p.TableName = p.Biz
	}
	if p.URLPath == "" {
		p.URLPath = "/" + p.Biz
	}
	// DataStruct must contain 'id', 'btime', 'mtime', 'seq' fields
	//   id: primary key
	//   btime: means birth time, the time when the doc created
	//   mtime: means modify time, the time when the doc modified
	//   seq: means the version of the doc
	p.FieldSet = BuildFieldSet(reflect.TypeOf(p.DataStruct))
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

	if p.Indexes != nil {
		for i := 0; i < len(p.Indexes); i++ {
			formatFields, err := p.FieldSet.CheckIndexFields(p.Indexes[i].Key)
			if err != nil {
				return fmt.Errorf("%s index[%v] check err: %s", p.Biz, p.Indexes[i].Key, err.Error())
			}
			p.Indexes[i].Key = formatFields
		}
	}

	p.FieldSet.SetCreateOnlyFields(p.CreateOnlyFields)
	p.FieldSet.SetReadOnlyFields(p.ReadOnlyFields)

	Log.Debugf("%v FieldSet %v", p.Biz, p.FieldSet)

	// default value
	if p.GetDbName == nil {
		p.GetDbName = p.defaultGetDbName()
	}
	if p.GetTableName == nil {
		p.GetTableName = p.defaultGetTableName()
	}
	if p.PostHandler == nil {
		p.PostHandler = p.defaultPost()
	}
	if p.PutHandler == nil {
		p.PutHandler = p.defaultPut()
	}
	if p.PatchHandler == nil {
		p.PatchHandler = p.defaultPatch()
	}
	if p.GetHandler == nil {
		p.GetHandler = p.defaultGet()
	}
	if p.GetPageHandler == nil {
		p.GetPageHandler = p.defaultGetPage()
	}
	if p.DeleteHandler == nil {
		p.DeleteHandler = p.defaultDelete()
	}
	if p.TriggerHandler == nil {
		p.TriggerHandler = p.defaultTrigger()
	}
	if p.OnWriteDone == nil {
		p.OnWriteDone = p.defaultOnWriteDone()
	}

	return nil
}

// Load is a function to register handlers
func (p *Processor) Load() {
	path := p.URLPath
	pathWithId := p.URLPath + "/{id}"
	pathWithTrigger := p.URLPath + "/__trigger"
	Register("POST", path, p.PostHandler)
	Register("PUT", pathWithId, p.PutHandler)
	Register("PATCH", pathWithId, p.PatchHandler)
	Register("GET", pathWithId, p.GetHandler)
	Register("GET", path, p.GetPageHandler)
	Register("DELETE", pathWithId, p.DeleteHandler)
	// TriggerHandler do something internal
	Register("POST", pathWithTrigger, p.TriggerHandler)
}

func (p *Processor) defaultGetDbName() func(query url.Values) string {
	return func(query url.Values) string {
		if db := query.Get("db"); db != "" {
			return db
		}
		if gCfg.DefaultDbName != "" {
			return gCfg.DefaultDbName
		}
		return "restful"
	}
}

func (p *Processor) defaultGetTableName() func(query url.Values) string {
	return func(query url.Values) string {
		if table := query.Get("table"); table != "" {
			return table
		}
		return p.TableName
	}
}

func (p *Processor) defaultPost() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		var err error
		var info map[string]interface{}
		if err = json.Unmarshal(body, &info); err != nil {
			Log.Warnf("POST %v unmarshal fail %v [%v]", p.URLPath, err, string(body))
			return genRsp(http.StatusBadRequest, "invalid Body", nil)
		}

		if id, ok := info["id"]; ok {
			v := GetString(id)
			if v == "" {
				Log.Warnf("POST %v custom id empty", p.URLPath)
				return genRsp(http.StatusBadRequest, "custom id empty", nil)
			}
			if len(v) > 128 {
				Log.Warnf("POST %v custom id too long", p.URLPath)
				return genRsp(http.StatusBadRequest, "custom id too long", nil)
			}
		} else {
			info["id"] = GenUniqueId()
		}

		err = p.FieldSet.CheckObject(info, false)
		if err != nil {
			Log.Warnf("POST %v invalid field exists, biz=%v err=%v", p.URLPath, p.Biz, err)
			return genRsp(http.StatusBadRequest, err.Error(), nil)
		}
		p.FieldSet.InReplace(&info)

		Log.Debugf("POST %v query=%v", p.URLPath, query)

		now := time.Now().Unix()
		info["btime"] = now
		info["mtime"] = now
		info["seq"] = genSeq(0)

		dbs := gCfg.MgoSess.Clone()
		defer dbs.Close()
		dbc := dbs.DB(p.GetDbName(query)).C(p.GetTableName(query))

		doc := p.FieldSet.InSort(&info)
		err = dbc.Insert(&doc)
		if err != nil {
			Log.Warnf("POST %v db access fail, err=%v", p.URLPath, err)
			if mgo.IsDup(err) {
				return genRsp(http.StatusBadRequest, "duplicate id", nil)
			}
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}

		if p.OnWriteDone != nil {
			go p.OnWriteDone("POST", vars, query, info)
		}
		// ensure index
		if p.Indexes != nil && len(p.Indexes) > 0 {
			getIndexEnsureList().Push(&indexToEnsureStruct{
				DB:        p.GetDbName(query),
				Table:     p.GetTableName(query),
				Processor: p,
			})
		}
		return genRsp(http.StatusOK, "post ok", map[string]interface{}{"id": info["_id"], "seq": info["seq"]})
	}
}

func (p *Processor) defaultPut() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		id := vars["id"]

		var err error
		var info map[string]interface{}
		if err = json.Unmarshal(body, &info); err != nil {
			Log.Warnf("PUT %v/%v unmarshal fail %v [%v]", p.URLPath, id, err, string(body))
			return genRsp(http.StatusBadRequest, "invalid Body", nil)
		}

		info["id"] = id
		if len(id) > 128 {
			Log.Warnf("PUT %v/%v id too long", p.URLPath, id)
			return genRsp(http.StatusBadRequest, "id too long", nil)
		}
		err = p.FieldSet.CheckObject(info, false)
		if err != nil {
			Log.Warnf("PUT %v/%v invalid field exists, biz=%v err=%v", p.URLPath, id, p.Biz, err)
			return genRsp(http.StatusBadRequest, err.Error(), nil)
		}
		p.FieldSet.InReplace(&info)

		Log.Debugf("PUT %v/%v query=%v", p.URLPath, id, query)

		now := time.Now().Unix()
		info["btime"] = now
		info["mtime"] = now
		info["seq"] = genSeq(0)

		dbs := gCfg.MgoSess.Clone()
		defer dbs.Close()
		dbc := dbs.DB(p.GetDbName(query)).C(p.GetTableName(query))

		var old map[string]interface{}
		err = dbc.Find(bson.M{"_id": id}).Select(bson.M{"btime": 1, "seq": 1}).One(&old)
		if err == nil {
			if v, ok := old["btime"]; ok {
				info["btime"] = v
			} else {
				info["btime"] = now
			}

			if v, ok := old["seq"]; ok {
				nextSeq, err2 := nextSeq(v.(string))
				if err2 == nil {
					info["seq"] = nextSeq
				} else {
					info["seq"] = genSeq(0)
				}
			}
		} else if err != mgo.ErrNotFound {
			Log.Warnf("PUT %v/%v db access fail, err=%v", p.URLPath, id, err)
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}

		doc := p.FieldSet.InSort(&info)
		_, err = dbc.Upsert(bson.M{"_id": id}, &doc)
		if err != nil {
			Log.Warnf("PUT %v/%v db access fail, err=%v", p.URLPath, id, err)
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}

		if p.OnWriteDone != nil {
			go p.OnWriteDone("PUT", vars, query, info)
		}
		// ensure index
		if p.Indexes != nil && len(p.Indexes) > 0 {
			getIndexEnsureList().Push(&indexToEnsureStruct{
				DB:        p.GetDbName(query),
				Table:     p.GetTableName(query),
				Processor: p,
			})
		}
		return genRsp(http.StatusOK, "put ok", map[string]interface{}{"id": info["_id"], "seq": info["seq"]})
	}
}

func (p *Processor) defaultPatch() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		id := vars["id"]

		var err error
		var info map[string]interface{}
		if err = json.Unmarshal(body, &info); err != nil {
			Log.Warnf("PATCH %v/%v unmarshal fail %v [%v]", p.URLPath, id, err, string(body))
			return genRsp(http.StatusBadRequest, "invalid Body", nil)
		}

		err = p.FieldSet.CheckObject(info, true)
		if err != nil {
			Log.Warnf("PATCH %v/%v invalid field exists, biz=%v err=%v", p.URLPath, id, p.Biz, err)
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
			Log.Warnf("PATCH %v/%v need seq", p.URLPath, id)
			return genRsp(http.StatusBadRequest, "need seq", nil)
		}

		now := time.Now().Unix()

		Log.Debugf("PATCH %v/%v query=%v", p.URLPath, id, query)

		dbs := gCfg.MgoSess.Clone()
		defer dbs.Close()
		dbc := dbs.DB(p.GetDbName(query)).C(p.GetTableName(query))

		if ignoreSeq {
			if _, ok := info["seq"]; ok {
				delete(info, "seq")
			}
			info["mtime"] = now
			err = dbc.Update(bson.M{"_id": id}, bson.M{"$set": info})
		} else {
			nextSeq, err2 := nextSeq(seq)
			if err2 != nil {
				Log.Warnf("PATCH %v/%v invalid seq: %s", p.URLPath, id, seq)
				return genRsp(http.StatusBadRequest, "invalid seq", nil)
			}
			info["seq"] = nextSeq
			info["mtime"] = now
			err = dbc.Update(bson.M{"_id": id, "seq": seq}, bson.M{"$set": info})
			if err == mgo.ErrNotFound {
				Log.Warnf("PATCH %v/%v id not found or seq conflict", p.URLPath, id)
				return genRsp(http.StatusBadRequest, "id not found or seq conflict", nil)
			}
		}

		if err != nil {
			Log.Warnf("PATCH %v/%v db access fail, err=%v", p.URLPath, id, err)
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}

		if p.OnWriteDone != nil {
			go p.OnWriteDone("PATCH", vars, query, info)
		}
		// ensure index
		if p.Indexes != nil && len(p.Indexes) > 0 {
			getIndexEnsureList().Push(&indexToEnsureStruct{
				DB:        p.GetDbName(query),
				Table:     p.GetTableName(query),
				Processor: p,
			})
		}
		if ignoreSeq {
			return genRsp(http.StatusOK, "patch ok", map[string]interface{}{"id": id})
		}
		return genRsp(http.StatusOK, "patch ok", map[string]interface{}{"id": id, "seq": info["seq"]})
	}
}

func (p *Processor) defaultGet() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		var err error
		id := vars["id"]

		// build select
		selector := make(map[string]interface{})
		if query.Get("select") != "" {
			var selSlice []string
			err := json.Unmarshal([]byte(query.Get("select")), &selSlice)
			if err != nil {
				Log.Warnf("GET %v/%v unmarshal select error: %v", p.URLPath, id, err)
				return genRsp(http.StatusBadRequest, "select invalid", nil)
			}
			err = p.FieldSet.BuildSelectObj(selSlice, selector)
			if err != nil {
				Log.Warnf("GET %v/%v select param invalid, %v", p.URLPath, id, err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		p.FieldSet.InReplace(&selector)

		Log.Debugf("GET %v/%v query=%v select=%v", p.URLPath, id, query, selector)

		// ensure index
		if p.Indexes != nil && len(p.Indexes) > 0 {
			getIndexEnsureList().Push(&indexToEnsureStruct{
				DB:        p.GetDbName(query),
				Table:     p.GetTableName(query),
				Processor: p,
			})
		}

		dbs := gCfg.MgoSess.Clone()
		defer dbs.Close()
		dbc := dbs.DB(p.GetDbName(query)).C(p.GetTableName(query))

		var info map[string]interface{}
		err = dbc.Find(bson.M{"_id": id}).Select(selector).One(&info)
		if err != nil {
			Log.Warnf("GET %v/%v get id=%s error, %v", p.URLPath, id, id, err)
			if err == mgo.ErrNotFound {
				return genRsp(http.StatusNotFound, "id not found", nil)
			}
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}
		p.FieldSet.OutReplace(&info)
		return genRsp(http.StatusOK, "get ok", info)
	}
}

func (p *Processor) defaultGetPage() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		var err error
		size := 0
		page := 0

		size, err = strconv.Atoi(query.Get("size"))
		if err != nil || (size <= 0 && size != -1) {
			Log.Warnf("GET %v size error", p.URLPath)
			return genRsp(http.StatusBadRequest, "need size or size invalid", nil)
		}

		page, err = strconv.Atoi(query.Get("page"))
		if err != nil || page <= 0 {
			Log.Warnf("GET %v page error", p.URLPath)
			return genRsp(http.StatusBadRequest, "need page or page invalid", nil)
		}

		// build condition
		condition := make(map[string]interface{})
		if query.Get("filter") != "" {
			var filter map[string]interface{}
			err := json.Unmarshal([]byte(query.Get("filter")), &filter)
			if err != nil {
				Log.Warnf("GET %v unmarshal filter error: %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, "filter invalid", nil)
			}
			err = p.FieldSet.BuildFilterObj(filter, condition)
			if err != nil {
				Log.Warnf("GET %v filter param invalid, %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		if query.Get("range") != "" {
			var rang map[string]interface{}
			err := json.Unmarshal([]byte(query.Get("range")), &rang)
			if err != nil {
				Log.Warnf("GET %v unmarshal range error: %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, "range invalid", nil)
			}
			err = p.FieldSet.BuildRangeObj(rang, condition)
			if err != nil {
				Log.Warnf("GET %v range param invalid, %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		if query.Get("in") != "" {
			var in map[string]interface{}
			err := json.Unmarshal([]byte(query.Get("in")), &in)
			if err != nil {
				Log.Warnf("GET %v unmarshal in error: %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, "in invalid", nil)
			}
			err = p.FieldSet.BuildInObj(in, condition)
			if err != nil {
				Log.Warnf("GET %v in param invalid, %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		if query.Get("nin") != "" {
			var nin map[string]interface{}
			err := json.Unmarshal([]byte(query.Get("nin")), &nin)
			if err != nil {
				Log.Warnf("GET %v unmarshal nin error: %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, "nin invalid", nil)
			}
			err = p.FieldSet.BuildNinObj(nin, condition)
			if err != nil {
				Log.Warnf("GET %v nin param invalid, %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		if query.Get("all") != "" {
			var all map[string]interface{}
			err := json.Unmarshal([]byte(query.Get("all")), &all)
			if err != nil {
				Log.Warnf("GET %v unmarshal all error: %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, "all invalid", nil)
			}
			err = p.FieldSet.BuildAllObj(all, condition)
			if err != nil {
				Log.Warnf("GET %v all param invalid, %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		if query.Get("or") != "" {
			var or []interface{}
			err := json.Unmarshal([]byte(query.Get("or")), &or)
			if err != nil {
				Log.Warnf("GET %v unmarshal or error: %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, "or invalid", nil)
			}
			err = p.FieldSet.BuildOrObj(or, condition)
			if err != nil {
				Log.Warnf("GET %v or param invalid, %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		if query.Get("search") != "" {
			search := query.Get("search")
			if search != "" {
				if !gCfg.EsEnable {
					Log.Warnf("GET %v search not config", p.URLPath)
					return genRsp(http.StatusInternalServerError, "search not config", nil)
				}
				ids, err := esSearch(p.GetDbName(query), p.GetTableName(query), search, 2000, 0)
				if err != nil {
					Log.Warnf("GET %v EsSearch err, %v", p.URLPath, err)
					return genRsp(http.StatusInternalServerError, err.Error(), nil)
				}
				if len(ids) == 0 {
					infos := make([]interface{}, 0)
					return genRsp(http.StatusOK, "no results found", RspGetPageData{Total: 0, Hits: infos})
				} else {
					if _, exist := condition["id"]; exist {
						Log.Warnf("GET %v search id condition conflict", p.URLPath)
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
				Log.Warnf("GET %v unmarshal order error: %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, "order invalid", nil)
			}
			err = p.FieldSet.BuildOrderArray(order, &sort)
			if err != nil {
				Log.Warnf("GET %v order param invalid, %v", p.URLPath, err)
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
				Log.Warnf("GET %v unmarshal select error: %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, "select invalid", nil)
			}
			err = p.FieldSet.BuildSelectObj(selSlice, selector)
			if err != nil {
				Log.Warnf("GET %v select param invalid, %v", p.URLPath, err)
				return genRsp(http.StatusBadRequest, err.Error(), nil)
			}
		}
		p.FieldSet.InReplace(&selector)

		Log.Debugf("GET %v query=%v condition=%v order=%v select=%v", p.URLPath, query, condition, orderFields, selector)

		// ensure index
		if p.Indexes != nil && len(p.Indexes) > 0 {
			getIndexEnsureList().Push(&indexToEnsureStruct{
				DB:        p.GetDbName(query),
				Table:     p.GetTableName(query),
				Processor: p,
			})
		}

		dbs := gCfg.MgoSess.Clone()
		defer dbs.Close()
		dbc := dbs.DB(p.GetDbName(query)).C(p.GetTableName(query))

		// count
		total := 0
		total, err = dbc.Find(condition).Count()
		if err != nil {
			Log.Warnf("GET %v get page count error: %v", p.URLPath, err)
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
			err = dbc.Find(condition).Skip(size * (page - 1)).Limit(size).Sort(orderFields...).Select(selector).All(&infos)
		default:
			err = fmt.Errorf("unknown")
		}
		if err != nil {
			Log.Warnf("GET %v get page results error: %v", p.URLPath, err)
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}

		p.FieldSet.OutReplaceArray(infos)
		return genRsp(http.StatusOK, "get page ok", RspGetPageData{Total: int64(total), Hits: infos})
	}
}

func (p *Processor) defaultDelete() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		var err error
		id := vars["id"]

		Log.Debugf("DELETE %v/%v query=%v", p.URLPath, id, query)

		dbs := gCfg.MgoSess.Clone()
		defer dbs.Close()
		dbc := dbs.DB(p.GetDbName(query)).C(p.GetTableName(query))

		err = dbc.Remove(bson.M{"_id": id})
		if err != nil {
			Log.Warnf("DELETE %v/%v delete id=%s error, %v", p.URLPath, id, err)
			if err == mgo.ErrNotFound {
				return genRsp(http.StatusNotFound, "id not found", nil)
			}
			return genRsp(http.StatusInternalServerError, "db access fail", nil)
		}

		if p.OnWriteDone != nil {
			go p.OnWriteDone("DELETE", vars, query, nil)
		}
		// ensure index
		if p.Indexes != nil && len(p.Indexes) > 0 {
			getIndexEnsureList().Push(&indexToEnsureStruct{
				DB:        p.GetDbName(query),
				Table:     p.GetTableName(query),
				Processor: p,
			})
		}
		return genRsp(http.StatusOK, "delete ok", map[string]interface{}{"id": id})
	}
}

func (p *Processor) defaultTrigger() Handler {
	return func(vars map[string]string, query url.Values, body []byte) *Rsp {
		Log.Debugf("POST %v/__trigger query=%v", p.URLPath, query)
		var err error
		var info map[string]interface{}
		if err = json.Unmarshal(body, &info); err != nil {
			Log.Warnf("POST %v/__trigger unmarshal fail %v [%v]", p.URLPath, err, string(body))
			return genRsp(http.StatusBadRequest, "invalid Body", nil)
		}

		typ := GetString(info["type"])
		if typ == "" {
			Log.Warnf("POST %v/__trigger trigger req need specified type", p.URLPath, err, string(body))
			return genRsp(http.StatusBadRequest, "need type", nil)
		}
		switch typ {
		case "search":
			id := GetString(info["id"])
			if id == "" {
				Log.Warnf("POST %v/__trigger search trigger req need specified id", p.URLPath, err, string(body))
				return genRsp(http.StatusBadRequest, "need id", nil)
			}
			if p.OnWriteDone != nil {
				vars = make(map[string]string)
				vars["id"] = id
				go p.OnWriteDone("PATCH", vars, query, nil)
			}
		default:
			Log.Warnf("POST %v/__trigger trigger type: %v unknown", p.URLPath, typ)
			return genRsp(http.StatusBadRequest, fmt.Sprintf("trigger type: %v unknown", typ), nil)
		}
		return genRsp(http.StatusOK, "trigger ok", nil)
	}
}

func (p *Processor) defaultOnWriteDone() func(method string, vars map[string]string, query url.Values, data map[string]interface{}) {
	return func(method string, vars map[string]string, query url.Values, data map[string]interface{}) {
		var err error
		db := p.GetDbName(query)
		table := p.GetTableName(query)
		switch method {
		case "POST":
			fallthrough
		case "PUT":
			if gCfg.EsEnable {
				id := GetString(data["_id"])
				content := p.FieldSet.BuildSearchContent(data, p.SearchFields)
				if content != "" {
					err = esUpsert(db, table, id, content)
				} else {
					err = esRemove(db, table, id)
				}
			}
		case "PATCH":
			if gCfg.EsEnable {
				dbs := gCfg.MgoSess.Clone()
				defer dbs.Close()
				dbc := dbs.DB(p.GetDbName(query)).C(p.GetTableName(query))
				id := vars["id"]
				var info map[string]interface{}
				err = dbc.Find(bson.M{"_id": id}).One(&info)
				if err != nil {
					Log.Warnf("OnWriteDone [%v][%v] db fail %v", p.Biz, method, err)
					return
				}
				content := p.FieldSet.BuildSearchContent(info, p.SearchFields)
				if content != "" {
					err = esUpsert(db, table, id, content)
				} else {
					err = esRemove(db, table, id)
				}
			}
		case "DELETE":
			if gCfg.EsEnable {
				id := vars["id"]
				err = esRemove(db, table, id)
			}
		}
		if err != nil {
			Log.Warnf("OnWriteDone [%v][%v] es access fail %v", p.Biz, method, err)
		}
	}
}
