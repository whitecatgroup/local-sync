package ipfs

import (
	"os/exec"
	"io/ioutil"
	"os"
	"strings"
	"errors"
	"bytes"
	"fmt"
	"encoding/json"
)

type Controller struct {
	ipfsPath string
	ipgetPath string
	daemonProcess *os.Process
}

type PersonalId struct {
	ID              string   `json:"ID"`
	PublicKey       string   `json:"PublicKey"`
	Addresses       []string `json:"Addresses"`
	AgentVersion    string   `json:"AgentVersion"`
	ProtocolVersion string   `json:"ProtocolVersion"`
}

func NewController(ipfsPath, ipgetPath string) *Controller {
	return &Controller{ipfsPath, ipgetPath, nil}
}

func genericError(err error, data []byte) error {
	return errors.New(err.Error() + " " + string(data))
}

func (c *Controller) GetResource(newName, address string) error {
	_, err := c.Execute(c.ipgetPath, []string{"-o", newName, address})
	return err
}

func (c *Controller) Execute(path string, args []string) ([]byte, error) {
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(path, args...)
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, errors.New(fmt.Sprint(err) + ": " + stderr.String())
	}
	return out.Bytes(), nil
}


func (c *Controller) PublishName(name string) error {
	data, err := c.Execute(c.ipfsPath, []string{"name", "publish", "/ipfs/" + name, "--local"})
	if err != nil {
		return err
	}
	output := string(data)
	if strings.Index(output, "Published to") < 0 {
		return errors.New("publishing error: " + output)
	}
	return nil
}

func (c *Controller) UploadFolder(path string) error {
	const mark = "added"
	const markLen = len(mark)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err
	}

	data, err := c.Execute(c.ipfsPath, []string{"add", "-r", path})
	if err != nil {
		return err
	}

	output := string(data)
	p := strings.LastIndex(output, "added")
	if p < 0 {
		return errors.New("processing error: " + output)
	}

	output = output[p + markLen + 1:]
	folderHashId := output[:strings.Index(output, " ")]
	return c.PublishName(folderHashId)
}

func (c *Controller) GetId() (*PersonalId, error) {
	data, err := c.Execute(c.ipfsPath, []string{"id"})
	if err != nil {
		return nil, err

	}
	id := &PersonalId{}
	return id, json.Unmarshal(data, id)
}

func (c *Controller) SaveBootstrapList(filename string) error {
	data, err := c.Execute(c.ipfsPath, []string{"bootstrap", "list"})
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, data, os.ModePerm)
}

func (c *Controller) LoadBootstrapList(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return c.SetBootstrapList(strings.Split(string(data), "\n"))
}

func (c *Controller) ClearBootstrapList() error {
	args := []string{"bootstrap", "rm", "--all"}
	output, err := c.Execute(c.ipfsPath, args)
	if err != nil {
		return genericError(err, output)
	}
	return nil
}

func (c *Controller) SetBootstrapList(list []string) error {
	err := c.ClearBootstrapList()
	if err != nil {
		return err
	}
	// add every bootstrap node to the ipfs
	args := []string{"bootstrap", "add", ""}
	for _, node := range list {
		if len(node) < 70 {
			continue
		}
		args[2] = node
		output, err := c.Execute(c.ipfsPath, args)
		if err != nil {
			return genericError(err, output)
		}
	}
	return nil
}

func (c *Controller) StartDaemon() error {
	cmd := exec.Command(c.ipfsPath, []string{"daemon"}...)
	err := cmd.Start()
	if err != nil {
		return err
	}
	c.daemonProcess = cmd.Process
	return nil
}

func (c *Controller) StopDaemon() error {
	if c.daemonProcess != nil {
		err := c.daemonProcess.Kill()
		if err != nil {
			return err
		}
		c.daemonProcess = nil
	}
	return nil
}