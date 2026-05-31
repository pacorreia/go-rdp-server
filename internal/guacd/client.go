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

	conn   net.Conn
	reader *bufio.Reader
}

// NewClient connects to guacd, performs the Guacamole protocol handshake for
// an RDP session, and then starts the background read/write goroutines.
// host, port, username, and password are the RDP connection parameters;
// values are sent positionally in the connect instruction after reading the
// args list that guacd returns.
func NewClient(ctx context.Context, addr, host, port, username, password string) (*Client, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	if err := handshake(conn, reader, host, port, username, password); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("guacd handshake: %w", err)
	}

	c := &Client{
		Send:   make(chan *Instruction, 64),
		Recv:   make(chan *Instruction, 64),
		conn:   conn,
		reader: reader,
	}

	go c.readLoop(ctx)
	go c.writeLoop(ctx)
	go func() {
		<-ctx.Done()
		_ = c.conn.Close()
	}()

	return c, nil
}

// handshake performs the Guacamole connection handshake on conn.
// It writes select, reads the args response, writes size/audio/video/image,
// then writes connect with positional values matching the args order.
func handshake(conn net.Conn, reader *bufio.Reader, host, port, username, password string) error {
	write := func(instr *Instruction) error {
		_, err := io.WriteString(conn, instr.Encode())
		return err
	}

	// 1. Select the RDP protocol.
	if err := write(&Instruction{Opcode: "select", Args: []string{"rdp"}}); err != nil {
		return fmt.Errorf("select: %w", err)
	}

	// 2. Read the args instruction guacd sends back.
	argsInstr, err := readInstruction(reader)
	if err != nil {
		return fmt.Errorf("read args: %w", err)
	}
	if argsInstr.Opcode != "args" {
		return fmt.Errorf("expected args instruction, got %q", argsInstr.Opcode)
	}

	// 3. Send tunnel capability negotiation instructions.
	for _, instr := range []*Instruction{
		{Opcode: "size", Args: []string{"1920", "1080", "96"}},
		{Opcode: "audio"},
		{Opcode: "video"},
		{Opcode: "image"},
	} {
		if err := write(instr); err != nil {
			return fmt.Errorf("%s: %w", instr.Opcode, err)
		}
	}

	// 4. Connect with positional values that match guacd's args order.
	known := map[string]string{
		"hostname": host,
		"port":     port,
		"username": username,
		"password": password,
	}
	connectArgs := make([]string, len(argsInstr.Args))
	for i, name := range argsInstr.Args {
		connectArgs[i] = known[name] // empty string for args we do not supply
	}
	return write(&Instruction{Opcode: "connect", Args: connectArgs})
}

func (c *Client) readLoop(ctx context.Context) {
	defer close(c.Recv)
	for {
		instruction, err := readInstruction(c.reader)
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
