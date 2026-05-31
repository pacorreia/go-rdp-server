package display

import (
	"bytes"
	"image/jpeg"
	"sync"

	"github.com/nakagami/grdp"
)

const (
	tileChanSize = 64
	jpegQuality  = 85
)

// Connect opens an RDP session to addr (host:port) using the provided
// credentials and requested display dimensions.
// domain may be empty for local accounts.
func Connect(addr, domain, username, password string, width, height int) (RDPSession, error) {
	s := &rdpSession{
		tiles: make(chan Tile, tileChanSize),
		done:  make(chan struct{}),
	}

	c := grdp.NewRdpClient(addr, width, height, nil)

	c.OnError(func(_ error) { s.closeDone() })
	c.OnClose(func() { s.closeDone() })
	c.OnBitmap(func(bitmaps []grdp.Bitmap) {
		for i := range bitmaps {
			b := &bitmaps[i]
			// RGBA() copies pixel data out of the pool-owned slice.
			// This call MUST stay inside the callback.
			rgba := b.RGBA()

			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, rgba, &jpeg.Options{Quality: jpegQuality}); err != nil {
				continue
			}
			tile := Tile{
				X:    b.DestLeft,
				Y:    b.DestTop,
				W:    b.Width,
				H:    b.Height,
				Data: buf.Bytes(),
			}
			if s.sendTile(tile) {
				return
			}
		}
	})

	s.c = c
	if err := c.Login(domain, username, password); err != nil {
		c.Close()
		s.closeDone()
		return nil, err
	}
	return s, nil
}

// rdpSession wraps a nakagami/grdp RdpClient as an RDPSession.
type rdpSession struct {
	c         *grdp.RdpClient
	tiles     chan Tile
	done      chan struct{}
	mu        sync.Mutex // guards closed and sends into tiles
	closed    bool
	closeOnce sync.Once
}

// sendTile delivers a tile to the consumer. It returns true when the session
// is done and the caller should stop sending.
func (s *rdpSession) sendTile(tile Tile) (done bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return true
	}
	select {
	case s.tiles <- tile:
	case <-s.done:
		return true
	default:
		// Drop tile when buffer is full to avoid blocking the RDP stream.
	}
	return false
}

func (s *rdpSession) closeDone() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		close(s.done)
		s.closed = true
		close(s.tiles)
		s.mu.Unlock()
	})
}

func (s *rdpSession) Tiles() <-chan Tile    { return s.tiles }
func (s *rdpSession) KeyDown(sc int)       { s.c.KeyDown(sc) }
func (s *rdpSession) KeyUp(sc int)         { s.c.KeyUp(sc) }
func (s *rdpSession) MouseMove(x, y int)   { s.c.MouseMove(x, y) }
func (s *rdpSession) MouseDown(b, x, y int) { s.c.MouseDown(b, x, y) }
func (s *rdpSession) MouseUp(b, x, y int)  { s.c.MouseUp(b, x, y) }
func (s *rdpSession) MouseWheel(d int)  { s.c.MouseWheel(d) }
func (s *rdpSession) Close() {
	s.c.Close()
	s.closeDone() // guarantee tiles is closed even if OnClose is never called
}
