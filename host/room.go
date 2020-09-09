package host

import (
	"sync"
	"encoding/json"
)

type Room struct {
	Members *Members		`json:"members"`
	mx sync.RWMutex 		`json:"-"`
}

type Members struct {
	RoomId string			`json:"room"`
	HostAddr string			`json:"host_addr"`
	HostPeerId string		`json:"host_id"`
	Nodes map[string]string `json:"nodes"`
}

func NewRoom(id string) *Room {
	return &Room{&Members{id, "", "", make(map[string]string)}, sync.RWMutex{}}
}

func (r *Room) Join(addr, peerId string) {
	r.mx.Lock()
	if len(r.Members.Nodes) < 1 {
		r.Members.HostAddr = addr
		r.Members.HostPeerId = peerId
	}
	r.Members.Nodes[addr] = peerId
	r.mx.Unlock()
}

func (r *Room) Leave(s string) {
	r.mx.Lock()
	delete(r.Members.Nodes, s)
	r.mx.Unlock()
}

func (r *Room) Get(s string) (string, bool) {
	r.mx.RLock()
	defer r.mx.RUnlock()
	s, ok := r.Members.Nodes[s]
	return s, ok
}

func (r *Room) SerializeNodes() ([]byte, error) {
	r.mx.RLock()
	defer r.mx.RUnlock()
	return json.Marshal(r.Members)
}