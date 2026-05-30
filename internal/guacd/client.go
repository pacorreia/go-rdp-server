package guacd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
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
		instruction, err := readInstruction(reader)
		if err != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case c.Recv <- instruction:
		}
	}
}

// readInstruction reads exactly one Guacamole instruction from r by following
// the length-prefixed framing. Unlike ReadString(';'), this correctly handles
// element values that contain ';' or ','.
func readInstruction(r *bufio.Reader) (*Instruction, error) {
	values := make([]string, 0, 8)
	for {
		// Read the length prefix up to and including the '.'
		lenStr, err := r.ReadString('.')
		if err != nil {
			return nil, err
		}
		// Strip the trailing '.'
		lenStr = lenStr[:len(lenStr)-1]
		n, err := strconv.Atoi(lenStr)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("invalid element length %q", lenStr)
		}

		// Read exactly n bytes for the element value.
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		values = append(values, string(buf))

		// Read the single-byte delimiter (',' or ';').
		delim, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		switch delim {
		case ',':
			// More elements follow.
		case ';':
			// End of instruction.
			if len(values) == 0 || values[0] == "" {
				return nil, errors.New("missing opcode")
			}
			return &Instruction{Opcode: values[0], Args: values[1:]}, nil
		default:
			return nil, fmt.Errorf("invalid delimiter %q", delim)
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
