package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/globalsign/mgo"
	"github.com/gorilla/mux"
	"github.com/jimdn/restful"
)

// step 1: init data structure
type Movie struct {
	Id       *string           `json:"id,omitempty" bson:"_id,omitempty"` // required
	Name     *string           `json:"name,omitempty" bson:"name,omitempty"`
	Year     *int32            `json:"year,omitempty" bson:"year,omitempty"`
	Director *string           `json:"director,omitempty" bson:"director,omitempty"`
	Actors   []string          `json:"actors,omitempty" bson:"actors,omitempty"`
	IsSequel *bool             `json:"is_sequel,omitempty" bson:"is_sequel,omitempty"`
	Comments []*Comment        `json:"comments,omitempty" bson:"comments,omitempty"`
	Extent1  map[string]string `json:"extent1,omitempty" bson:"extent1,omitempty"`
	Extent2  map[string]int64  `json:"extent2,omitempty" bson:"extent2,omitempty"`
	Btime    *int64            `json:"btime,omitempty" bson:"btime,omitempty"` // internal, doc birth time
	Mtime    *int64            `json:"mtime,omitempty" bson:"mtime,omitempty"` // internal, doc modify time
	Seq      *string           `json:"seq,omitempty" bson:"seq,omitempty"`     // internal, doc seq, prevent concurrent updating
}
type Comment struct {
	UserId  *string  `json:"user_id,omitempty" bson:"user_id,omitempty"`
	Score   *float64 `json:"score,omitempty" bson:"score,omitempty"`
	Content *string  `json:"content,omitempty" bson:"content,omitempty"`
}

func main() {
	// step 2: init router and mongodb
	router := mux.NewRouter()
	mgoSess, err := mgo.DialWithTimeout("mongodb://127.0.0.1:27017", 5*time.Second)
	if err != nil {
		fmt.Printf("mongo dial err: %v\n", err)
		return
	}

	// step 3: init restful
	processors := []restful.Processor{
		{
			Biz:              "movie",
			URLPath:          "/movie",
			DataStruct:       new(Movie),
			SearchFields:     []string{"id", "name", "director", "actors"}, // fields for searching
			CreateOnlyFields: []string{"director", "actors"},               // fields cannot be modified after created
		},
	}
	restfulCfg := restful.GlobalConfig{
		Mux:              router,
		MgoSess:          mgoSess,
		EsEnable:         true,
		EsUrl:            "http://127.0.0.1:9200",
		EsUser:           "",
		EsPwd:            "",
		EsIndex:          "restful",
		EsAnalyzer:       "ik_max_word", // analyzer plugins
		EsSearchAnalyzer: "ik_max_word", // search analyzer plugins
	}
	err = restful.Init(&restfulCfg, &processors)
	if err != nil {
		fmt.Printf("restful init err: %v\n", err)
		return
	}

	// step4: start http server
	srv := &http.Server{
		Handler:      router,
		Addr:         "127.0.0.1:8080",
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}
	err = srv.ListenAndServe()
	if err != nil {
		fmt.Printf("serve err: %v", err)
	}
}
