package tcp_client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/labstack/gommon/log"
)

type KanataTcpClient struct {
	ClientMessageCh chan ClientMessage
	serverMessageCh chan ServerMessage
}

func NewTcpClient() *KanataTcpClient {
	c := &KanataTcpClient{
		ClientMessageCh: make(chan ClientMessage),
		serverMessageCh: make(chan ServerMessage),
	}
	return c
}

// Connects to kanata and runs message loop.
//
// Passed ctx controls lifetime of all goroutines of the TCP client;
// it's unlike ctx in Dial() (which is just for initial connection)
//
// initialConnectErr returns nil on connection success, otherwise error.
func (c *KanataTcpClient) StartInBackground(parentCtx context.Context, port int, initialConnTimeout time.Duration) (initialConnectErr chan error) {
	initialConnectErr = make(chan error, 1)
	go func() {
		loop := true
		firstLoop := true
		for loop {
			tmp := firstLoop
			firstLoop = false
			firstLoop := tmp
			func() {
				ctx, cancel := context.WithCancel(parentCtx)
				defer cancel()

				var connEstablishTimeout = time.Millisecond * 100
				if firstLoop {
					connEstablishTimeout = initialConnTimeout
				}
				conn, err := c.shortpollConnect(ctx, port, time.Millisecond*50, connEstablishTimeout)
				if err != nil {
					initialConnectErr <- fmt.Errorf("shortpollConnect after %s: %w", connEstablishTimeout, err)
					loop = false
					return
				}
				if firstLoop {
					initialConnectErr <- nil
				}

				// TX Loop
				go func() {
					for {
						select {
						case <-ctx.Done():
							return
						case msg := <-c.ClientMessageCh:
							msgBytes := msg.Bytes()
							_, err := conn.Write(msgBytes)
							if err != nil {
								log.Errorf("tcp client: failed to send message: %v", err)
								// TODO: better handle failure here? maybe try reconnect?
							} else {
								log.Debugf("msg sent: %s", string(msgBytes))
							}
						}
					}
				}()

				// RX Loop
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					var msgBytes = scanner.Bytes()
					// do not change the following condition (because of cross-version compability)
					if bytes.Contains(msgBytes, []byte("you sent an invalid message")) {
						log.Errorf("Kanata disconnected us because we supposedly sent an 'invalid message' (kanata version is too old?)")
						loop = true
						return
					}
					var msg ServerMessage
					err := json.Unmarshal(msgBytes, &msg)
					if err != nil {
						log.Errorf("tcp client: failed to json-unmarshal message '%s': %v", string(msgBytes), err)
						continue
					}
					c.serverMessageCh <- msg // TODO: should this be non-blocking to fix edge cases?
				}
				if err := scanner.Err(); err != nil {
					loop = false
					if !errors.Is(ctx.Err(), context.Canceled) {
						log.Errorf("tcp client: failed to read stream: %v", err)
					}
				}
			}()
		}
	}()

	return initialConnectErr
}

// Non-nil error means success.
func (c *KanataTcpClient) shortpollConnect(ctx context.Context, port int, attemptTimeout time.Duration, establishTimeout time.Duration) (net.Conn, error) {

	tcpWaitDeadline := time.Now().Add(establishTimeout)
	var lastErr error = context.DeadlineExceeded
	for time.Now().Before(tcpWaitDeadline) {
		connStart := time.Now()
		conn, err := c.connFn(ctx, port, attemptTimeout)
		if err == nil {
			return conn, nil
		} else if errors.Is(err, context.Canceled) {
			return nil, context.Canceled
		} else { // likely context.DeadlineExceeded
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Now().Add(attemptTimeout).Sub(connStart)):
		}
	}
	return nil, lastErr
}

func (_ *KanataTcpClient) connFn(ctx context.Context, port int, connectTimeout time.Duration) (net.Conn, error) {
	conn, err := (&net.Dialer{
		Timeout: connectTimeout,
	}).DialContext(ctx, "tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, err
	}

	log.Infof("Connected to kanata via TCP (%s)", conn.LocalAddr().String())

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	return conn, nil
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
