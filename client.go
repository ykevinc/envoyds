package envoyds

import (
	"bytes"
	"fmt"
	"github.com/golang/protobuf/jsonpb"
	"log"
	"net/http"
	"sync"
	"time"
)

const CLIENT_PERIOD = time.Second * 20

type DsClient struct {
	dsIp               string
	dsPort             int
	ownIp              string
	ownPort            int
	service            string
	done               chan bool
	lock               *sync.Mutex
	httpRegisterString string
}

func NewClient(dsIp string, dsPort int, ownIp string, ownPort int, service string) *DsClient {
	marshaler := jsonpb.Marshaler{EmitDefaults: true, OrigName: true}
	postRequest := ServicePostRequest{}
	postRequest.Ip = ownIp
	postRequest.Port = int32(ownPort)
	registerString, err := marshaler.MarshalToString(&postRequest)
	if err != nil {
		log.Println(err)
	}
	return &DsClient{dsIp: dsIp, dsPort: dsPort, ownIp: ownIp, ownPort: ownPort, service: service, done: nil, lock: &sync.Mutex{}, httpRegisterString: registerString}
}

func (c *DsClient) Start() {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.done != nil {
		c.Stop()
	}
	c.done = make(chan bool, 1)
	go func() {
		c.Register()
		ticker := time.NewTicker(CLIENT_PERIOD)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.Register()
			case <-c.done:
				return
			}
		}
	}()
}

func (c *DsClient) Stop() {
	c.lock.Lock()
	defer c.lock.Unlock()
	close(c.done)
	c.done = nil
}

func (c *DsClient) Register() {
	reader := bytes.NewBufferString(c.httpRegisterString)
	log.Printf("Register %s:%d to discovery service at %s:%d\n", c.ownIp, c.ownPort, c.dsIp, c.dsPort)
	resp, err := http.Post(fmt.Sprintf("http://%s:%d/v1/registration/%s", c.dsIp, c.dsPort, c.service), "application/json; charset=utf-8", reader)
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println(err)
	}
}