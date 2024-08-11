package tcp_client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/labstack/gommon/log"
)

type KanataTcpClient struct {
	ClientMessageCh chan ClientMessage
	Reconnect       chan struct{}

	serverMessageCh chan ServerMessage // shouldn't be written to from outside

	mu     sync.Mutex // allow only 1 conn at a time
	conn   net.Conn
	dialer net.Dialer
}

func NewTcpClient() *KanataTcpClient {
	c := &KanataTcpClient{
		ClientMessageCh: make(chan ClientMessage),
		Reconnect:       make(chan struct{}, 1),
		serverMessageCh: make(chan ServerMessage),
		mu:              sync.Mutex{},
		dialer: net.Dialer{
			Timeout: time.Second * 3,
		},
	}
	return c
}

func (c *KanataTcpClient) Connect(ctx context.Context, port int) error {
	c.mu.Lock()
	var err error
	c.conn, err = c.dialer.DialContext(ctx, "tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		c.mu.Unlock()
		return err
	}
	log.Infof("Connected to kanata via TCP (%s)", c.conn.LocalAddr().String())
	ctxSend, cancelSenderLoop := context.WithCancel(ctx)
	go func() {
		for {
			select {
			case <-ctxSend.Done():
				return
			case msg := <-c.ClientMessageCh:
				msgBytes := msg.Bytes()
				_, err := c.conn.Write(msgBytes)
				if err != nil {
					log.Errorf("tcp client: failed to send message: %v", err)
				} else {
					log.Debugf("msg sent: %s", string(msgBytes))
				}
			}
		}
	}()
	go func() {
		defer c.mu.Unlock()
		defer cancelSenderLoop()
		scanner := bufio.NewScanner(c.conn)
		for scanner.Scan() {
			var msgBytes = scanner.Bytes()
			// do not change the following condition (because of cross-version compability)
			if bytes.Contains(msgBytes, []byte("you sent an invalid message")) {
				log.Errorf("Kanata disconnected us because we supposedly sent an 'invalid message' (kanata version is too old?)")
				c.Reconnect <- struct{}{}
				return
			}
			var msg ServerMessage
			err := json.Unmarshal(msgBytes, &msg)
			if err != nil {
				log.Errorf("tcp client: failed to unmarshal message '%s': %v", string(msgBytes), err)
				continue
			}
			c.serverMessageCh <- msg
		}
		if err := scanner.Err(); err != nil {
			log.Errorf("tcp client: failed to read stream: %v", err)
		}
	}()
	return nil
}

func (c *KanataTcpClient) ServerMessageCh() <-chan ServerMessage {
	return c.serverMessageCh
}

type ClientMessage struct {
	RequestLayerNames struct{} `json:"RequestLayerNames"`
}

func (c *ClientMessage) Bytes() []byte {
	msgBytes, err := json.Marshal(c)
	if err != nil {
		panic(fmt.Sprintf("tcp client: failed to marshal ClientMessage '%v'\n", c))
	}
	return msgBytes
}

// ==================

type ServerMessage struct {
	LayerChange      *LayerChange      `json:"LayerChange"`
	LayerNames       *LayerNames       `json:"LayerNames"`
	ConfigFileReload *ConfigFileReload `json:"ConfigFileReload"`
}

// {"LayerChange":{"new":"newly-changed-to-layer"}}
type LayerChange struct {
	NewLayer string `json:"new"`
}

type LayerNames struct {
	Names []string `json:"names"`
}

type ConfigFileReload struct {
	New string `json:"new"`
}
