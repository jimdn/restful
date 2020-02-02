package restful

import (
	"errors"
	"github.com/globalsign/mgo"
	"github.com/gorilla/mux"
)

type GlobalConfig struct {
	Mux                *mux.Router  // gorilla/mux
	MgoSess            *mgo.Session // mongodb session
	MgoDefaultDbPrefix string       // mongodb default db prefix, using "rest_" if not setting
	MgoDefaultCol      string       // mongodb default collection name, using "cn" if not setting
	EsEnable           bool         // enable es for search
	EsUrl              string       // es url, default: http://127.0.0.1:9200
	EsUser             string       // es username
	EsPwd              string       // es password
	EsIndex            string       // es index, default: restful
	EsAnalyzer         string       // default: ik_max_word
	EsSearchAnalyzer   string       // default: ik_max_word
}

var gCfg GlobalConfig

func Init(cfg *GlobalConfig, processors *[]Processor) error {
	if cfg == nil || cfg.Mux == nil || cfg.MgoSess == nil {
		return errors.New("cfg param invalid")
	}
	if processors == nil || len(*processors) == 0 {
		return errors.New("processors param invalid")
	}

	gCfg = *cfg
	if gCfg.EsEnable {
		err := InitEsParam(gCfg.EsUrl, gCfg.EsUser, gCfg.EsPwd, gCfg.EsIndex, gCfg.EsAnalyzer, gCfg.EsSearchAnalyzer)
		if err != nil {
			return err
		}
	}

	for i := 0; i < len(*processors); i++ {
		p := &(*processors)[i]
		err := p.Init()
		if err != nil {
			return err
		}
		p.Load()
	}
	return nil
}
