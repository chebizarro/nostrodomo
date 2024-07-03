package nips

type Nip interface {
	ConnectionListen()
	ConnectionReceive()
	ConnectionPublish()
	ConnectionSubscribe()
	ConnectionUnSubscribe()
	RelayConnect()
	RelayDisconnect()
	RelayPublish()
	RelaySubscribe()
}