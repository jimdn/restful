package main

import (
	"fmt"
	"github.com/globalsign/mgo"
	"github.com/gorilla/mux"
	"github.com/jimdn/restful"
	"net/http"
	"time"
)

// step 1: init data structure
type Teacher struct {
	Id      *string `json:"id,omitempty" bson:"_id,omitempty"` // required
	Name    *string `json:"name,omitempty" bson:"name,omitempty"`
	Age     *int64  `json:"age,omitempty" bson:"age,omitempty"`
	Sex     *string `json:"sex,omitempty" bson:"sex,omitempty"`
	Subject *string `json:"subject,omitempty" bson:"subject,omitempty"`
	Btime   *int64  `json:"btime,omitempty" bson:"btime,omitempty"` // internal, doc birth time
	Mtime   *int64  `json:"mtime,omitempty" bson:"mtime,omitempty"` // internal, doc modify time
	Seq     *string `json:"seq,omitempty" bson:"seq,omitempty"`     // internal, doc seq, prevent concurrent updating
}
type Student struct {
	Id      *string  `json:"id,omitempty" bson:"_id,omitempty"` // required
	Name    *string  `json:"name,omitempty" bson:"name,omitempty"`
	Age     *int64   `json:"age,omitempty" bson:"age,omitempty"`
	Sex     *string  `json:"sex,omitempty" bson:"sex,omitempty"`
	Hobbies []string `json:"hobbies,omitempty" bson:"hobbies,omitempty"`
	Btime   *int64   `json:"btime,omitempty" bson:"btime,omitempty"` // internal, doc birth time
	Mtime   *int64   `json:"mtime,omitempty" bson:"mtime,omitempty"` // internal, doc modify time
	Seq     *string  `json:"seq,omitempty" bson:"seq,omitempty"`     // internal, doc seq, prevent concurrent updating
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
			Biz:        "teachers",
			URLPath:    "/teachers",
			DataStruct: new(Teacher),
		},
		{
			Biz:        "students",
			URLPath:    "/students",
			DataStruct: new(Student),
		},
	}
	restfulCfg := restful.GlobalConfig{
		Mux:     router,
		MgoSess: mgoSess,
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
