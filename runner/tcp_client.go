package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

type KanataTcpClient struct {
	ServerMessageCh chan ServerMessage

	mu     sync.Mutex // allow only 1 conn at a time
	conn   net.Conn
	dialer net.Dialer
}

func NewTcpClient() *KanataTcpClient {
	c := &KanataTcpClient{
		ServerMessageCh: make(chan ServerMessage),
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
	go func() {
		defer c.mu.Unlock()
		scanner := bufio.NewScanner(c.conn)
		for scanner.Scan() {
			var msgBytes = scanner.Bytes()
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
			// todo: restart maybe, if not ctx error?
		}
	}()
	return nil
}

type ServerMessage struct {
	LayerChange *LayerChange `json:"LayerChange"`
}

// {"LayerChange":{"new":"newly-changed-to-layer"}}
type LayerChange struct {
	NewLayer string `json:"new"`
}
