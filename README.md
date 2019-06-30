# jimdn/restful
A package based on Golang and MongoDB for quickly building HTTP Restful services with JSON.

[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/jimdn/restful?style=flat-square)](https://goreportcard.com/report/github.com/jimdn/restful)

## Required

- go 1.10+
- mongodb v3.4.x v3.6.x
- elasticsearch v6.7 v7.x (if enable searching)

## Dependencies:

- github.com/gorilla/mux
- github.com/globalsign/mgo
- github.com/nu7hatch/gouuid

## Feature Overview

- Define the structure of the data resource (including json and bson2 tags), then you can implement the CURD service of HTTP+JSON. The protocol is as follows:

| HTTP Method | Path | URL params | Explain |
|------|-----|------|-----|
| POST | /{biz} | - | insert data |
| PUT | /{biz}/{id} | - | insert or update(overwrite) data by id |
| PATCH | /{biz}/{id} | seq | update data by id |
| DELETE | /{biz}/{id} | - | delete data by id |
| GET | /{biz}/{id} | - | get data by id |
| GET | /{biz} | page<br/> size<br/>  filter<br/>  range<br/>  in<br/> nin<br/> all<br/> search<br/>  order<br/>select | get list of data:<br/>page=1<br/>size=10<br/>filter={"star":5, "city":"shenzhen"}<br/>range={"age":{"gt":20, "lt":40}}<br/>in={"keywords":["blue", "red"]}<br/>nin={"keywords":["blue", "red"]}<br/>all={"keywords":["blue", "red"]}<br/>search=hello<br/>order=["+age", "-time"]<br/>select=["id", "name", "age"]<br/>|

- When defining a data resource structure, the supported data types include:
  ```bash
  common types: bool int32 uint32 int64 uint64 float32 float64 string struct
  array types: []bool []int32 []uint32 []int64 []uint64 []float32 []float64 []string []struct
  map types: map[string]bool map[string]int32 map[string]uint32 map[string]int64 map[string]uint64 map[string]float32 map[string]float64 map[string]string  map[string]struct
  ```
- Support field level `CreateOnly` or `ReadOnly`:
  - CreateOnly: only allows creation, does not allow subsequent modification of the field
  - ReadOnly: only allows reading, does not allow creation and modification, is suitable for importing data from other systems to the database, and then providing data reading services.

- With the field check function, the incoming data field type is wrong or does not exist, it will return a failure and prompt specific error information.

- Support custom data ID or automatically create ID (UUIDv4), BTW, pay attention to the writing of tags:
  ```go
    type Foo struct {
        Id  *string  `json:"id,omitempty" bson:"_id,omitempty"`
        ...
    }
   ```

- Support tracking data birth time and modify time, two additional fields required:
  - btime: birth time, record the timestamp when data created
  - mtime: modify time, record the timestamp the last modification of data

- Support anti-concurrent writing, the `seq` field required:
  - seq: will be updated each time the data is modified, the update (PATCH) request needs to bring the data original seq to prevent concurrent writing from causing data confusion.

- Support custom database name and collection name, with URL params:
  - db: database name, default is rest_{Biz}
  - col: collection name, default is cn
  e.g.: /{Biz}?db=dbName&col=colName

## How to use
see example
