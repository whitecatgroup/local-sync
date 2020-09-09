package host

import (
	"github.com/go-playground/lars"
	"github.com/spaolacci/murmur3"
	"encoding/binary"
	"encoding/hex"
	"time"
	"strconv"
	"net/http"
	"github.com/orcaman/concurrent-map"
)

type OnUpdateEvent func(addr string) bool

var (
	rooms = cmap.New()
	updateEvent OnUpdateEvent
)

func roomHash() string {
	//now := time.Now()
	//mm3 := murmur3.New128WithSeed(uint32(now.Hour()) * uint32(now.Nanosecond()))
	mm3 := murmur3.New128()
	mm3.Write([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
	h1, h2 := mm3.Sum128()
	b1 := make([]byte, 8)
	b2 := make([]byte, 8)
	binary.LittleEndian.PutUint64(b1, h1)
	binary.LittleEndian.PutUint64(b2, h2)
	b1 = append(b1, b2...)
	return hex.EncodeToString(b1)
}

func RoomMembers(c lars.Context) {
	id := c.Param("id")
	r := c.Response()
	if len(id) < 10 {
		r.WriteHeader(http.StatusBadRequest)
		r.WriteString("invalid room")
		return
	}
	if i, ok := rooms.Get(id); ok {
		data, err := i.(*Room).SerializeNodes()
		if err != nil {
			r.WriteHeader(http.StatusInternalServerError)
			r.WriteString("room is inaccessible")
			return
		}
		r.Header().Set("Content-Type", "application/json")
		r.WriteHeader(http.StatusOK)
		r.Write(data)
		return
	}
	r.WriteHeader(http.StatusNotFound)
	r.WriteString("room not found")
}

func CreateRoom(c lars.Context) {
	h := roomHash()
	r := c.Response()
	rooms.Set(h, NewRoom(h))
	r.WriteHeader(http.StatusOK)
	r.WriteString(h)
}

func UpdateRoom(c lars.Context) {
	r := c.Response()
	id := c.Param("id")
	if len(id) < 10 {
		r.WriteHeader(http.StatusBadRequest)
		r.WriteString("invalid room")
		return
	}
	addr := c.Request().Header.Get("Room-sync-id")
	if len(addr) < 70 {
		r.WriteHeader(http.StatusBadRequest)
		r.WriteString("invalid personal address " + addr)
		return
	}
	if i, ok := rooms.Get(id); ok {
		if peerId, ok := i.(*Room).Get(addr); ok {
			if updateEvent != nil && updateEvent(peerId) {
				r.WriteHeader(http.StatusOK)
				r.WriteString("ok")
				return
			}
		}
	}
	r.WriteHeader(http.StatusBadRequest)
	r.WriteString("invalid room")
}

func JoinRoom(c lars.Context) {
	r := c.Response()
	id := c.Param("id")
	if len(id) < 10 {
		r.WriteHeader(http.StatusBadRequest)
		r.WriteString("invalid room")
		return
	}
	peerId := c.Param("peer")
	if len(peerId) < 10 {
		r.WriteHeader(http.StatusBadRequest)
		r.WriteString("invalid peer id")
		return
	}
	addr := c.Request().Header.Get("Room-sync-id")
	if len(addr) < 70 {
		r.WriteHeader(http.StatusBadRequest)
		r.WriteString("invalid personal address")
		return
	}
	if i, ok := rooms.Get(id); ok {
		i.(*Room).Join(addr, peerId)
		r.WriteHeader(http.StatusOK)
		r.WriteString("ok")
		return
	}
	r.WriteHeader(http.StatusBadRequest)
	r.WriteString("invalid room")
}