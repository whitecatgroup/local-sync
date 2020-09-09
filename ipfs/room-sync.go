package ipfs

import (
	"net/http"
	"time"
	"io/ioutil"
	"errors"
	"encoding/json"
	"fmt"
	"strings"
	"os"
	"path/filepath"
)

var (
	InvalidIdErr       = errors.New("can't retrieve IPFS id. Perhaps, daemon is not running")
	NoMembersErr       = errors.New("members were not cached previously")
	AddressesNotSetErr = errors.New("no valid address located")
)

type RoomSync struct {
	c 			*Controller
	client 		*http.Client
	baseUrl 	string
	currentRoom string
	id 			*PersonalId
	lastMembers *Members
}

type Members struct {
	RoomId 		string			`json:"room"`
	HostAddr 	string			`json:"host_addr"`
	HostPeerId 	string			`json:"host_id"`
	Nodes 		map[string]string `json:"nodes"`
}

func NewRoomSync(ipfsPath, ipgetPath, hostUrl string) (*RoomSync, error) {
	c := NewController(ipfsPath, ipgetPath)
	tr := &http.Transport {
		MaxIdleConns:       10,
		IdleConnTimeout:    10 * time.Second,
		DisableCompression: true,
	}
	client := &http.Client{Transport: tr}
	return &RoomSync{c, client, hostUrl + "/v1/", "", nil, &Members{}}, nil
}

func (rs *RoomSync) clearDirectory(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(path, name))
		if err != nil {
			return err
		}
	}
	return nil
}

func (rs *RoomSync) request(route, method, syncHeader string) ([]byte, error) {
	url := rs.baseUrl + route
	req, err := http.NewRequest(method, url, nil)
	req.Header.Set("Room-sync-id", syncHeader)
	req.Header.Set("Content-Type", "application/json")
	res, err := rs.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return ioutil.ReadAll(res.Body)
}

func (rs *RoomSync) GetLastMembers() *Members {
	return rs.lastMembers
}

func (rs *RoomSync) UpdatePeerId() error {
	id, err := rs.c.GetId()
	if err != nil {
		return err
	}
	if id.Addresses == nil || len(id.Addresses) < 1 {
		return InvalidIdErr
	}
	rs.id = id
	return nil
}

func (rs *RoomSync) GetController() *Controller {
	return rs.c
}

func (rs *RoomSync) GetPersonalAddress() (string, error) {
	if rs.id == nil {
		return "", InvalidIdErr
	}
	addresses := rs.id.Addresses
	for _, a := range addresses {
		a2 := a[5:]
		if a[1:4] == "ip4" && !(strings.HasPrefix(a2, "127.0.0.1") || strings.HasPrefix(a2, "localhost")) {
			return a, nil
		}
	}
	return "", AddressesNotSetErr
}

func (rs * RoomSync) CreateRoom() (string, error) {
	addr, err := rs.GetPersonalAddress()
	if err != nil {
		return "", err
	}
	result, err := rs.request("room", "POST", addr)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func (rs *RoomSync) JoinRoom(id string) error {
	peer, err := rs.c.GetId()
	if err != nil {
		return err
	}
	addr, err := rs.GetPersonalAddress()
	if err != nil {
		return err
	}
	result, err := rs.request("room/" + id + "/" + peer.ID, "POST", addr)
	if err != nil {
		return err
	}
	resultStr := string(result)
	if resultStr != "ok" {
		return errors.New("unknown result " + resultStr)
	}
	rs.currentRoom = id
	return nil
}

func (rs *RoomSync) UpdateRoomMembers() error {
	result, err := rs.request("room/" + rs.currentRoom, "GET", "")
	if err != nil {
		return err
	}
	err = json.Unmarshal(result, rs.lastMembers)
	if err != nil {
		return err
	}
	bootstrapList := make([]string, 0)
	for addr, peerId := range rs.lastMembers.Nodes {
		if peerId == rs.id.ID {
			continue
		}
		bootstrapList = append(bootstrapList, addr)
		fmt.Println("Peer added", peerId)
	}
	return rs.c.SetBootstrapList(bootstrapList)
}

func (rs *RoomSync) Sync(path string) error {
	if rs.lastMembers == nil {
		return NoMembersErr
	}
	err := rs.clearDirectory(path)
	if err != nil {
		return err
	}
	return rs.c.GetResource(path, "/ipns/" + rs.lastMembers.HostPeerId)
}

func (rs *RoomSync) SyncRemote(path, peerId string) error {
	if rs.lastMembers == nil {
		return NoMembersErr
	}
	err := rs.clearDirectory(path)
	if err != nil {
		return err
	}
	return rs.c.GetResource(path, "/ipns/" + peerId)
}

func (rs *RoomSync) Upload(path string) error {
	if rs.lastMembers == nil {
		return NoMembersErr
	}
	err := rs.c.UploadFolder(path)
	if err != nil {
		return err
	}
	addr, err := rs.GetPersonalAddress()
	if err != nil {
		return err
	}
	data, err := rs.request("room/" + rs.currentRoom, "POST", addr)
	if err != nil {
		return err
	}
	output := string(data)
	if output != "ok" {
		return errors.New("updating error: " + output)
	}
	return nil
}

func (rs *RoomSync) GetId() *PersonalId {
	return rs.id
}