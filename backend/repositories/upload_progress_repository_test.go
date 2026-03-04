package repositories

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeRedisServer struct {
	ln       net.Listener
	mu       sync.Mutex
	sets     map[string]map[string]struct{}
	expires  map[string]time.Time
	shutdown chan struct{}
}

func startFakeRedisServer(t *testing.T) *fakeRedisServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start fake redis listener failed: %v", err)
	}

	srv := &fakeRedisServer{
		ln:       ln,
		sets:     make(map[string]map[string]struct{}),
		expires:  make(map[string]time.Time),
		shutdown: make(chan struct{}),
	}

	go srv.serve()
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

func (s *fakeRedisServer) Addr() string {
	return s.ln.Addr().String()
}

func (s *fakeRedisServer) Close() error {
	select {
	case <-s.shutdown:
		return nil
	default:
		close(s.shutdown)
		return s.ln.Close()
	}
}

func (s *fakeRedisServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return
			default:
				return
			}
		}
		go s.handleConn(conn)
	}
}

func (s *fakeRedisServer) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		cmd, err := readRESPArray(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			_ = writeError(writer, "ERR protocol error")
			_ = writer.Flush()
			return
		}
		if len(cmd) == 0 {
			_ = writeError(writer, "ERR empty command")
			_ = writer.Flush()
			continue
		}

		s.execCommand(strings.ToUpper(cmd[0]), cmd[1:], writer)
		if err := writer.Flush(); err != nil {
			return
		}
	}
}

func (s *fakeRedisServer) execCommand(cmd string, args []string, writer *bufio.Writer) {
	switch cmd {
	case "HELLO":
		_ = writeError(writer, "ERR unknown command 'HELLO'")
		return
	case "CLIENT":
		_ = writeSimpleString(writer, "OK")
		return
	case "PING":
		_ = writeSimpleString(writer, "PONG")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked()

	switch cmd {
	case "SADD":
		if len(args) < 2 {
			_ = writeError(writer, "ERR wrong number of arguments for 'sadd'")
			return
		}
		key := args[0]
		if _, ok := s.sets[key]; !ok {
			s.sets[key] = make(map[string]struct{})
		}
		added := int64(0)
		for _, member := range args[1:] {
			if _, ok := s.sets[key][member]; ok {
				continue
			}
			s.sets[key][member] = struct{}{}
			added++
		}
		_ = writeInteger(writer, added)
	case "SISMEMBER":
		if len(args) != 2 {
			_ = writeError(writer, "ERR wrong number of arguments for 'sismember'")
			return
		}
		key := args[0]
		member := args[1]
		if _, ok := s.sets[key][member]; ok {
			_ = writeInteger(writer, 1)
		} else {
			_ = writeInteger(writer, 0)
		}
	case "SCARD":
		if len(args) != 1 {
			_ = writeError(writer, "ERR wrong number of arguments for 'scard'")
			return
		}
		key := args[0]
		_ = writeInteger(writer, int64(len(s.sets[key])))
	case "SMEMBERS":
		if len(args) != 1 {
			_ = writeError(writer, "ERR wrong number of arguments for 'smembers'")
			return
		}
		key := args[0]
		members := make([]string, 0, len(s.sets[key]))
		for member := range s.sets[key] {
			members = append(members, member)
		}
		sort.Strings(members)
		_ = writeBulkStringArray(writer, members)
	case "EXPIRE":
		if len(args) != 2 {
			_ = writeError(writer, "ERR wrong number of arguments for 'expire'")
			return
		}
		key := args[0]
		seconds, err := strconv.Atoi(args[1])
		if err != nil {
			_ = writeError(writer, "ERR value is not an integer")
			return
		}
		if _, ok := s.sets[key]; !ok {
			_ = writeInteger(writer, 0)
			return
		}
		s.expires[key] = time.Now().Add(time.Duration(seconds) * time.Second)
		_ = writeInteger(writer, 1)
	case "DEL":
		if len(args) == 0 {
			_ = writeError(writer, "ERR wrong number of arguments for 'del'")
			return
		}
		deleted := int64(0)
		for _, key := range args {
			if _, ok := s.sets[key]; ok {
				delete(s.sets, key)
				delete(s.expires, key)
				deleted++
			}
		}
		_ = writeInteger(writer, deleted)
	default:
		_ = writeError(writer, fmt.Sprintf("ERR unknown command '%s'", strings.ToLower(cmd)))
	}
}

func (s *fakeRedisServer) cleanupExpiredLocked() {
	now := time.Now()
	for key, expireAt := range s.expires {
		if now.After(expireAt) {
			delete(s.expires, key)
			delete(s.sets, key)
		}
	}
}

func readRESPArray(reader *bufio.Reader) ([]string, error) {
	header, err := readLine(reader)
	if err != nil {
		return nil, err
	}
	if len(header) == 0 || header[0] != '*' {
		return nil, fmt.Errorf("invalid array header: %q", header)
	}

	count, err := strconv.Atoi(header[1:])
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		bulkHeader, err := readLine(reader)
		if err != nil {
			return nil, err
		}
		if len(bulkHeader) == 0 || bulkHeader[0] != '$' {
			return nil, fmt.Errorf("invalid bulk header: %q", bulkHeader)
		}

		size, err := strconv.Atoi(bulkHeader[1:])
		if err != nil {
			return nil, err
		}
		payload := make([]byte, size+2)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, err
		}
		result = append(result, string(payload[:size]))
	}

	return result, nil
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

func writeSimpleString(writer *bufio.Writer, value string) error {
	_, err := writer.WriteString("+" + value + "\r\n")
	return err
}

func writeError(writer *bufio.Writer, value string) error {
	_, err := writer.WriteString("-" + value + "\r\n")
	return err
}

func writeInteger(writer *bufio.Writer, value int64) error {
	_, err := writer.WriteString(":" + strconv.FormatInt(value, 10) + "\r\n")
	return err
}

func writeBulkStringArray(writer *bufio.Writer, values []string) error {
	if _, err := writer.WriteString("*" + strconv.Itoa(len(values)) + "\r\n"); err != nil {
		return err
	}
	for _, value := range values {
		if _, err := writer.WriteString("$" + strconv.Itoa(len(value)) + "\r\n"); err != nil {
			return err
		}
		if _, err := writer.WriteString(value + "\r\n"); err != nil {
			return err
		}
	}
	return nil
}

func TestUploadChunkKey(t *testing.T) {
	got := uploadChunkKey("abc")
	want := "upload:abc:chunks"
	if got != want {
		t.Fatalf("uploadChunkKey mismatch, got %q want %q", got, want)
	}
}

func TestTimeDurationSeconds(t *testing.T) {
	got := timeDurationSeconds(12)
	want := 12 * time.Second
	if got != want {
		t.Fatalf("timeDurationSeconds mismatch, got %v want %v", got, want)
	}
}

func TestRedisUploadProgressRepository_EndToEnd(t *testing.T) {
	srv := startFakeRedisServer(t)
	client := redis.NewClient(&redis.Options{
		Addr:            srv.Addr(),
		Protocol:        2,
		DisableIdentity: true,
	})
	t.Cleanup(func() { _ = client.Close() })

	repo := NewRedisUploadProgressRepository(client)
	ctx := context.Background()

	exists, err := repo.IsChunkUploaded(ctx, "u1", 1)
	if err != nil {
		t.Fatalf("IsChunkUploaded initial failed: %v", err)
	}
	if exists {
		t.Fatalf("expected chunk not uploaded initially")
	}

	if err := repo.AddChunk(ctx, "u1", 1, 60); err != nil {
		t.Fatalf("AddChunk 1 failed: %v", err)
	}
	if err := repo.AddChunk(ctx, "u1", 2, 0); err != nil {
		t.Fatalf("AddChunk 2 failed: %v", err)
	}

	exists, err = repo.IsChunkUploaded(ctx, "u1", 1)
	if err != nil {
		t.Fatalf("IsChunkUploaded after add failed: %v", err)
	}
	if !exists {
		t.Fatalf("expected chunk to be uploaded")
	}

	count, err := repo.UploadedCount(ctx, "u1")
	if err != nil {
		t.Fatalf("UploadedCount failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected uploaded count 2, got %d", count)
	}

	if err := client.SAdd(ctx, uploadChunkKey("u1"), "invalid-number").Err(); err != nil {
		t.Fatalf("SAdd invalid-number failed: %v", err)
	}

	chunks, err := repo.UploadedChunks(ctx, "u1")
	if err != nil {
		t.Fatalf("UploadedChunks failed: %v", err)
	}
	sort.Ints(chunks)
	if len(chunks) != 2 || chunks[0] != 1 || chunks[1] != 2 {
		t.Fatalf("expected chunks [1 2], got %v", chunks)
	}

	if err := repo.Clear(ctx, "u1"); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	count, err = repo.UploadedCount(ctx, "u1")
	if err != nil {
		t.Fatalf("UploadedCount after clear failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected uploaded count 0 after clear, got %d", count)
	}
}

func TestRedisUploadProgressRepository_AddChunk_WithExpiry(t *testing.T) {
	srv := startFakeRedisServer(t)
	client := redis.NewClient(&redis.Options{
		Addr:            srv.Addr(),
		Protocol:        2,
		DisableIdentity: true,
	})
	t.Cleanup(func() { _ = client.Close() })

	repo := NewRedisUploadProgressRepository(client)
	ctx := context.Background()

	if err := repo.AddChunk(ctx, "u-expire", 8, 1); err != nil {
		t.Fatalf("AddChunk with expiry failed: %v", err)
	}

	time.Sleep(1200 * time.Millisecond)

	count, err := repo.UploadedCount(ctx, "u-expire")
	if err != nil {
		t.Fatalf("UploadedCount after expiry failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected uploaded count 0 after expiry, got %d", count)
	}
}
