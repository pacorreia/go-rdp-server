package guacd

import (
	"bufio"
	"context"
	"io"
	"net"
)

// Client is a channel-based TCP client for guacd.
type Client struct {
	Send chan *Instruction
	Recv chan *Instruction

	conn net.Conn
}

// NewClient connects to guacd and starts read/write goroutines.
func NewClient(ctx context.Context, addr string) (*Client, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	c := &Client{
		Send: make(chan *Instruction, 64),
		Recv: make(chan *Instruction, 64),
		conn: conn,
	}

	go c.readLoop(ctx)
	go c.writeLoop(ctx)
	go func() {
		<-ctx.Done()
		_ = c.conn.Close()
	}()

	return c, nil
}

func (c *Client) readLoop(ctx context.Context) {
	defer close(c.Recv)
	reader := bufio.NewReader(c.conn)
	for {
		raw, err := reader.ReadString(';')
		if err != nil {
			return
		}

		instruction, err := Decode(raw)
		if err != nil {
			continue
		}

		select {
		case <-ctx.Done():
			return
		case c.Recv <- instruction:
		}
	}
}

func (c *Client) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case instruction, ok := <-c.Send:
			if !ok {
				return
			}
			if instruction == nil {
				continue
			}
			if _, err := io.WriteString(c.conn, instruction.Encode()); err != nil {
				return
			}
		}
	}
}
