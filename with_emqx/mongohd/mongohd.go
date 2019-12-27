package mongohd

import (
	"cloudserver/with_emqx/utils"

	"time"

	"gopkg.in/mgo.v2"
)

var (
	_dialinfo = &mgo.DialInfo{
		Addrs:     []string{utils.CfgInstance.Mongo.Host + ":" + utils.CfgInstance.Mongo.Port},
		Direct:    false,
		Timeout:   10 * time.Second,
		Database:  utils.CfgInstance.Mongo.Database,
		Source:    utils.CfgInstance.Mongo.AuthSource,
		Username:  utils.CfgInstance.Mongo.User,
		Password:  utils.CfgInstance.Mongo.Pwd,
		PoolLimit: 30}

	_session *mgo.Session = nil
)

func ConnectMongo() error {
	sess, err := mgo.DialWithInfo(_dialinfo)
	if err == nil {
		_session = sess
	}
	return err
}

//结构是通过http告知其他模块，设备状态等暂不是这边管理。mongo留作备用
