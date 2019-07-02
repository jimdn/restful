# jimdn/restful
A package based on Golang and MongoDB for quickly building HTTP RESTful services with JSON.

[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/jimdn/restful?style=flat-square)](https://goreportcard.com/report/github.com/jimdn/restful)

## Required

- go 1.10+
- mongodb v3.4.x v3.6.x
- elasticsearch v6.7 v7.x (if enable searching)

## Dependencies

- github.com/gorilla/mux
- github.com/globalsign/mgo
- github.com/nu7hatch/gouuid

## Feature Overview

- Define the structure of the data resource (including json and bson tags), then you can implement the CURD service of HTTP+JSON. The protocol is as follows:

| HTTP Method | Path | URL Params | HTTP Body | Explain |
|------|-----|------|-----|-----|
| POST | /{biz} | - | data to be inserted | insert data |
| PUT | /{biz}/{id} | - |  data to be upserted | insert or update(overwrite) data by id |
| PATCH | /{biz}/{id} | seq |  data to be updated | update data by id |
| DELETE | /{biz}/{id} | - |  - | delete data by id |
| GET | /{biz}/{id} | - |  - | get data by id |
| GET | /{biz} | page<br/> size<br/>  filter<br/>  range<br/>  in<br/> nin<br/> all<br/> search<br/>  order<br/>select |  - | get list of data:<br/>page=1<br/>size=10<br/>filter={"star":5, "city":"shenzhen"}<br/>range={"age":{"gt":20, "lt":40}}<br/>in={"color":["blue", "red"]}<br/>nin={"color":["blue", "red"]}<br/>all={"color":["blue", "red"]}<br/>search=hello<br/>order=["+age", "-time"]<br/>select=["id", "name", "age"]<br/>|

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
See [examples](examples). We take the `Student` of [simple.go](examples/simple.go) as an example:

### Insert resource (with or without id)
Request:
```http
POST /students HTTP/1.1
Content-Type: application/json; charset=utf-8
Content-Length: 226

{
    "id": "student-id-001",
    "name": "jimmydeng",
    ...
}
```

Response:
```http
HTTP/1.1 200 OK
Date: Mon, 22 Apr 2019 06:46:23 GMT
Content-Type: application/json; charset=utf-8
Content-Length: 91

{
    "code": 0,
    "msg": "post ok",
    "data": {
        "id": "student-id-001"
    }
}
```


### Upsert resource by id
Request:
```http
PUT /students/student-id-001 HTTP/1.1
Content-Type: application/json; charset=utf-8
Content-Length: 226

{
    "id": "student-id-001",
    "name": "jimmydeng",
    ...
}
```

Response:
```http
HTTP/1.1 200 OK
Date: Mon, 22 Apr 2019 06:46:23 GMT
Content-Type: application/json; charset=utf-8
Content-Length: 90

{
    "code": 0,
    "msg": "put ok",
    "data": {
        "id": "student-id-001"
    }
}
```


### Update resource by id
Request:
```http
PATCH /students/student-id-001?seq=1 HTTP/1.1
Content-Type: application/json; charset=utf-8
Content-Length: 226

{
    "id": "student-id-001",
    "name": "jimmydeng02",
    ...
}
```

Response:
```http
HTTP/1.1 200 OK
Date: Mon, 22 Apr 2019 06:46:23 GMT
Content-Type: application/json; charset=utf-8
Content-Length: 30

{
    "code": 0,
    "msg": "patch ok",
    "data": {
        "id": "student-id-001"
    }
}
```


### Delete resource by id
Request:
```http
DELETE /students/student-id-001 HTTP/1.1
```

Response:
```http
HTTP/1.1 200 OK
Date: Mon, 22 Apr 2019 06:46:23 GMT
Content-Type: application/json; charset=utf-8
Content-Length: 30

{
    "code": 0,
    "msg": "delete ok",
    "data": {
        "id": "student-id-001"
    }
}
```

### Get resource by id
Request:
```http
GET /students/student-id-001 HTTP/1.1
```

Response:
```http
HTTP/1.1 200 OK
Date: Mon, 22 Apr 2019 06:46:23 GMT
Content-Type: application/json; charset=utf-8
Content-Length: 537

{
    "code": 0,
    "msg": "get ok",
    "data": {
        "id": "student-id-001"
        "name": "jimmydeng",
        ...
    }
}
```

### Get resources
Request:
```http
GET /students?page=1&size=10 HTTP/1.1
```

Response:
```http
HTTP/1.1 200 OK
Date: Mon, 22 Apr 2019 06:46:23 GMT
Content-Type: application/json; charset=utf-8
Content-Length: 797

{
    "code": 0,
    "msg": "get page ok",
    "data": {
        "total": 238,
        "hits": [
            {
                "id": "student-id-001",
                "name": "jimmydeng",
                ...
            },
            {
                "id": "student-id-002",
                "name": "tonywho",
                ...
            }
            ...
        ]
    }
}
```
