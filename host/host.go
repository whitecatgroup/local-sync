package host

import (
	"github.com/go-playground/lars"
	"fmt"
	"net/http"
	"local-sync/ipfs"
	"errors"
	"time"
	"log"
)

type DiscoveryHost struct {
	syncInProgress bool
	rs             *ipfs.RoomSync
	server         *http.Server
	updatePath     string
}

func NewDiscoveryHost(rs *ipfs.RoomSync) *DiscoveryHost {
	return &DiscoveryHost{false, rs, &http.Server{}, ""}
}

func (host *DiscoveryHost) stopSync() {
	host.syncInProgress = false
}

func (host *DiscoveryHost) syncOver(addr string) {
	defer host.stopSync()
	err := host.rs.SyncRemote(host.updatePath, addr)
	if err != nil {
		fmt.Println("- sync failed due", err.Error())
		return
	}
	// published recent files
	err = host.rs.GetController().UploadFolder(host.updatePath)
	if err != nil {
		fmt.Println("- sync.upload failed due", err.Error())
	}
}

func (host *DiscoveryHost) sync(addr string) bool {
	if host.syncInProgress {
		return false
	}
	host.syncInProgress = true
	go host.syncOver(addr)
	return true
}

func (host *DiscoveryHost) Shutdown() error {
	if host.syncInProgress {
		return errors.New("sync in progress")
	}
	return host.server.Close()
}

func (host *DiscoveryHost) Start() {
	updateEvent = host.sync

	router := lars.New()

	apiV1 := router.Group("/v1")
	apiV1.Use(Logger)

	room := apiV1.Group("/room")
	room.Get("/:id", RoomMembers)
	room.Post("/:id", UpdateRoom)
	room.Post("", CreateRoom)
	room.Post("/", CreateRoom)
	room.Post("/:id/:peer", JoinRoom)

	host.server.Addr = ":3000"
	host.server.Handler = router.Serve()
	host.server.ListenAndServe()
}

func (host *DiscoveryHost) UpdateWorkingPath(p string) {
	host.updatePath = p
}

func Logger(c lars.Context) {
	start := time.Now()

	c.Next()

	stop := time.Now()
	path := c.Request().URL.Path

	if path == "" {
		path = "/"
	}

	log.Printf("%s %d %s %s", c.Request().Method, c.Response().Status(), path, stop.Sub(start))
}