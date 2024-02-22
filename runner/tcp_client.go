package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

type KanataTcpClient struct {
	ServerMessageCh chan ServerMessage // shouldn't be written to from outside

	clientMessageCh chan ClientMessage

	reconnect chan struct{}

	mu     sync.Mutex // allow only 1 conn at a time
	conn   net.Conn
	dialer net.Dialer
}

func NewTcpClient() *KanataTcpClient {
	c := &KanataTcpClient{
		ServerMessageCh: make(chan ServerMessage),
		clientMessageCh: make(chan ClientMessage),
		reconnect:       make(chan struct{}, 1),
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
	fmt.Printf("Connected to kanata via TCP (%s)\n", c.conn.LocalAddr().String())
	ctxSend, cancelSenderLoop := context.WithCancel(ctx)
	go func() {
		for {
			select {
			case <-ctxSend.Done():
				return
			case msg := <-c.clientMessageCh:
				msgBytes := msg.Bytes()
				_, err := c.conn.Write(msgBytes)
				if err != nil {
					fmt.Printf("tcp client: failed to send message: %v\n", err)
				}
				// else {
				// fmt.Printf("msg sent: %s\n", string(msgBytes))
				// }
			}
		}
	}()
	go func() {
		defer c.mu.Unlock()
		defer cancelSenderLoop()
		scanner := bufio.NewScanner(c.conn)
		for scanner.Scan() {
			var msgBytes = scanner.Bytes()
			if bytes.HasPrefix(msgBytes, []byte("you sent an invalid message")) {
				fmt.Printf("Kanata disconnected us because we supposedly sent an 'invalid message' (kanata version is too old?)\n")
				c.reconnect <- struct{}{}
				return
			}
			var msg ServerMessage
			err := json.Unmarshal(msgBytes, &msg)
			if err != nil {
				fmt.Printf("tcp client: failed to unmarshal message '%s': %v\n", string(msgBytes), err)
				continue
			}
			c.ServerMessageCh <- msg
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("tcp client: failed to read stream: %v\n", err)
		}
	}()
	return nil
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
	LayerChange *LayerChange `json:"LayerChange"`
	LayerNames  *LayerNames  `json:"LayerNames"`
}

// {"LayerChange":{"new":"newly-changed-to-layer"}}
type LayerChange struct {
	NewLayer string `json:"new"`
}

type LayerNames struct {
	Names []string `json:"names"`
}
