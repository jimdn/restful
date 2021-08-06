package restful

import (
	"errors"
	"fmt"
	"github.com/globalsign/mgo"
	"github.com/gorilla/mux"
)

// GlobalConfig is a config to init restful service
type GlobalConfig struct {
	Mux                *mux.Router  // gorilla/mux
	MgoSess            *mgo.Session // mongodb session
	DefaultDbName      string       // default db name, using "restful" if not setting
	DefaultIdGenerator string       // default id gnerator, objectid or uuid, using objectid if not setting
	EsEnable           bool         // enable es for search
	EsUrl              string       // es url, default: http://127.0.0.1:9200
	EsUser             string       // es username
	EsPwd              string       // es password
	EsIndex            string       // es index, default: restful
	EsAnalyzer         string       // default: ik_max_word
	EsSearchAnalyzer   string       // default: ik_max_word
}

var gCfg GlobalConfig

// Init is a function to init restful service
func Init(cfg *GlobalConfig, processors *[]Processor) error {
	if cfg == nil || cfg.Mux == nil || cfg.MgoSess == nil {
		return errors.New("cfg param invalid")
	}
	if processors == nil || len(*processors) == 0 {
		return errors.New("processors param invalid")
	}

	gCfg = *cfg
	if gCfg.DefaultIdGenerator == "" {
		gCfg.DefaultIdGenerator = "objectid"
	}
	if gCfg.EsEnable {
		err := initEsParam(gCfg.EsUrl, gCfg.EsUser, gCfg.EsPwd, gCfg.EsIndex, gCfg.EsAnalyzer, gCfg.EsSearchAnalyzer)
		if err != nil {
			return err
		}
	}

	bizMap := make(map[string]bool)
	for i := 0; i < len(*processors); i++ {
		p := &(*processors)[i]
		if _, ok := bizMap[p.Biz]; ok {
			return fmt.Errorf("biz: %s conflict", p.Biz)
		}
		bizMap[p.Biz] = true

		err := p.Init()
		if err != nil {
			return err
		}
		p.Load()
	}

	go ensureIndexTask()
	return nil
}
