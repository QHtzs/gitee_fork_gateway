package mqttclient

import (
	"cloudserver/with_emqx/utils"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

var _client mqtt.Client = nil

func CreatClient() mqtt.Client {
	opts := mqtt.NewClientOptions().AddBroker("tcp://" + utils.CfgInstance.Mqtt.Host + ":" + utils.CfgInstance.Mqtt.Port)
	opts.SetUsername(utils.CfgInstance.Mqtt.User)
	opts.SetPassword(utils.CfgInstance.Mqtt.Pwd)
	opts.SetClientID(utils.CfgInstance.Mqtt.ClientId)
	opts.SetCleanSession(true)
	opts.SetConnectRetryInterval(10 * time.Second)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() == nil {
		return client
	} else {
		utils.Logger.Println("mqtt 登录失败", token.Error())
		return nil
	}
}

func ReConnect() {
	_client = CreatClient()
}

//默认其为并发安全的
func Publish(topic string, qos byte, retained bool, payload interface{}) /* bool*/ {
	// token := _client.Publish(topic, qos, retained, payload)
	// token.WaitTimeout(3 * time.Second)
	// if token.Error() != nil {
	// 	utils.Logger.Println(token.Error())
	// }
	// return token.Error() == nil
	if _client == nil {
		ReConnect()
	} else {
		_client.Publish(topic, qos, retained, payload)
	}
}

func Subsribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	if _client == nil {
		ReConnect()
		return nil
	} else {
		return _client.Subscribe(topic, qos, callback)
	}
}
