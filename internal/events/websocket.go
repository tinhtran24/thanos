package events

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

func ServeWebSocket(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		key := r.Header.Get("Sec-WebSocket-Key")
		if key == "" || r.Header.Get("Upgrade") == "" {
			http.Error(w, "websocket upgrade required", http.StatusBadRequest)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "websocket unsupported", http.StatusInternalServerError)
			return
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			return
		}
		if err := writeHandshake(rw, key); err != nil {
			_ = conn.Close()
			return
		}
		events, unsubscribe := hub.Subscribe(32)
		defer unsubscribe()
		defer conn.Close()

		for event := range events {
			payload, err := Marshal(event)
			if err != nil {
				continue
			}
			if err := writeTextFrame(conn, payload); err != nil {
				return
			}
		}
	}
}

func writeHandshake(rw *bufio.ReadWriter, key string) error {
	hash := sha1.Sum([]byte(key + websocketGUID))
	accept := base64.StdEncoding.EncodeToString(hash[:])
	if _, err := fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(rw, "Upgrade: websocket\r\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(rw, "Connection: Upgrade\r\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(rw, "Sec-WebSocket-Accept: %s\r\n\r\n", accept); err != nil {
		return err
	}
	return rw.Flush()
}

func writeTextFrame(conn net.Conn, payload []byte) error {
	header := []byte{0x81}
	switch {
	case len(payload) < 126:
		header = append(header, byte(len(payload)))
	case len(payload) <= 65535:
		header = append(header, 126, byte(len(payload)>>8), byte(len(payload)))
	default:
		header = append(header, 127,
			byte(uint64(len(payload))>>56),
			byte(uint64(len(payload))>>48),
			byte(uint64(len(payload))>>40),
			byte(uint64(len(payload))>>32),
			byte(uint64(len(payload))>>24),
			byte(uint64(len(payload))>>16),
			byte(uint64(len(payload))>>8),
			byte(uint64(len(payload))),
		)
	}
	if _, err := conn.Write(header); err != nil {
		return err
	}
	_, err := conn.Write(payload)
	return err
}
