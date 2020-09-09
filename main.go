package main

import (
	"fmt"
	"flag"
	"os"
	"time"
	"bufio"
	"strings"
	"local-sync/host"
	"local-sync/ipfs"
)

const MaxSyncAwaitTime = 5 * time.Second

var workingDirectory string

func assert(err error) {
	if err != nil {
		panic("Error: " + err.Error())
	}
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func setWorkingDir(reader *bufio.Reader, discoveryHost *host.DiscoveryHost) error {
	fmt.Print("\nEnter a path to a folder: ")
	path, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	path = strings.TrimSpace(path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err
	}
	workingDirectory = path
	if discoveryHost != nil {
		discoveryHost.UpdateWorkingPath(path)
	}
	fmt.Println("New working directory is", path)
	return nil
}

func main() {
	ipfsPtr := flag.String("ipfs", "ipfs", "a path to ipfs")
	ipgetPtr := flag.String("ipget", "ipget", "a path to ipget")
	hostAddrPtr := flag.String("host", "localhost:3100", "a host address")
	createPtr := flag.Bool("create", false, "create a new room and join it")
	roomPtr := flag.String("room", "", "a room id")
	manualDaemon := flag.Bool("daemon", false, "run a local daemon instance")
	flag.Parse()

	if ipfsPtr == nil || !fileExists(*ipfsPtr) {
		if ipfsPtr == nil {
			fmt.Println("Provide -ipfs path")
		} else {
			fmt.Println("Invalid ipfs path:", *ipfsPtr)
		}
		return
	}

	if ipgetPtr == nil || !fileExists(*ipgetPtr) {
		if ipgetPtr == nil {
			fmt.Println("Provide -ipget path")
		} else {
			fmt.Println("Invalid ipget path:", *ipgetPtr)
		}
		return
	}

	if !(createPtr != nil && *createPtr) && (hostAddrPtr == nil || len(*hostAddrPtr) < 10) {
		if hostAddrPtr == nil {
			fmt.Println("Provide -host address")
		} else {
			fmt.Println("Invalid host address:", *hostAddrPtr)
		}
		return
	}

	roomSync, err := ipfs.NewRoomSync(*ipfsPtr, *ipgetPtr, "http://" + *hostAddrPtr)
	assert(err)

	fmt.Println("Starting the service...")
	c := roomSync.GetController()
	// if an instance of IPFS daemon required
	daemonRequired := manualDaemon != nil && *manualDaemon
	if daemonRequired {
		assert(c.StartDaemon())
		time.Sleep(time.Second * 3)
	}
	assert(roomSync.UpdatePeerId())

	var roomId string
	var discoveryHost *host.DiscoveryHost

	if createPtr != nil && *createPtr {
		fmt.Println("Hosting...")
		discoveryHost = host.NewDiscoveryHost(roomSync)
		go discoveryHost.Start()
		time.Sleep(time.Second * 1)
		roomId, err = roomSync.CreateRoom()
		assert(err)
		fmt.Println("New room id", roomId)
	}

	if len(roomId) < 10 {
		if roomPtr == nil || len(*roomPtr) < 10 {
			fmt.Println("Invalid room id!", roomId)
			return
		}
		roomId = *roomPtr
	}

	id := roomSync.GetId()
	fmt.Println("Your id", id.ID)
	fmt.Println("Your addresses", id.Addresses)

	fmt.Println("Joining room", roomId)
	err = roomSync.JoinRoom(roomId)
	assert(err)

	assert(roomSync.UpdateRoomMembers())

	fmt.Println("Commands: 'room', 'sync', 'upload', 'dir', 'exit'")

	reader := bufio.NewReader(os.Stdin)
	run := true
	for run {
		fmt.Print("\nEnter a command: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		switch text {
		case "room":
			fmt.Print("\n'update' or 'members': ")
			text, _ = reader.ReadString('\n')
			text = strings.TrimSpace(text)
			if text == "update" {
				err = roomSync.UpdateRoomMembers()
				if err != nil {
					fmt.Println("- room update failed due", err.Error())
				} else {
					fmt.Println("ok")
				}
			} else if text == "members" {
				members := roomSync.GetLastMembers()
				i := 0
				for k, v := range members.Nodes {
					i++
					fmt.Println(i, k, v)
				}
			}
			break
		case "sync":
			if workingDirectory == "" {
				err = setWorkingDir(reader, discoveryHost)
				if err != nil {
					fmt.Println("- invalid working directory:", err.Error())
					break
				}
			}
			err = roomSync.Sync(workingDirectory)
			if err != nil {
				fmt.Println("- sync failed due", err.Error())
			}
			break
		case "upload":
			if workingDirectory == "" {
				err = setWorkingDir(reader, discoveryHost)
				if err != nil {
					fmt.Println("- invalid working directory:", err.Error())
					break
				}
			}
			err = roomSync.Upload(workingDirectory)
			if err != nil {
				fmt.Println("- upload failed due", err.Error())
			}
			break
		case "dir":
			err = setWorkingDir(reader, discoveryHost)
			if err != nil {
				fmt.Println("- invalid working directory:", err.Error())
			}
			break
		case "exit", "q":
			run = false
			break
		}
	}

	if daemonRequired {
		defer c.StartDaemon()
	}

	if discoveryHost != nil {
		attempts := 0
		for discoveryHost.Shutdown() != nil {
			attempts++
			if attempts > 5 {
				return
			}
			fmt.Println("Awaiting for the synchronization to end...")
			time.Sleep(MaxSyncAwaitTime)
		}
	}
}