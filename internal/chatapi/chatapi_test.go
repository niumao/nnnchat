package chatapi

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// trackedConn wraps one end of a net.Pipe so tests can observe whether
// AddClient called Close on it, without relying on a bytes.Buffer (which
// would EOF immediately and make readLoop close the connection on its own,
// independent of anything AddClient decided).
type trackedConn struct {
	net.Conn
	closed chan struct{}
}

func (t *trackedConn) Close() error {
	select {
	case <-t.closed:
	default:
		close(t.closed)
	}
	return t.Conn.Close()
}

// newHandshakeConn returns a tracked server-side connection with the given
// JSON handshake already written to it (so AddClient's decode succeeds),
// and the matching client-side end, which the test must keep open for the
// duration it wants the connection to look "alive".
func newHandshakeConn(t *testing.T, room, name, token string) (*trackedConn, net.Conn) {
	t.Helper()
	data, err := json.Marshal(clientInfo{Room: room, Name: name, Token: token})
	if err != nil {
		t.Fatalf("could not marshal handshake: %v", err)
	}
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close() })
	go client.Write(data)
	return &trackedConn{Conn: server, closed: make(chan struct{})}, client
}

func signToken(t *testing.T, secret []byte, subject string, rooms []string, ttl time.Duration) string {
	t.Helper()
	claims := struct {
		Rooms []string `json:"rooms"`
		jwt.RegisteredClaims
	}{
		Rooms: rooms,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(secret)
	if err != nil {
		t.Fatalf("could not sign test token: %v", err)
	}
	return s
}

func assertNotClosed(t *testing.T, conn *trackedConn) {
	t.Helper()
	select {
	case <-conn.closed:
		t.Fatal("AddClient closed the connection, expected it to stay open")
	case <-time.After(50 * time.Millisecond):
	}
}

func assertClosed(t *testing.T, conn *trackedConn) {
	t.Helper()
	select {
	case <-conn.closed:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("AddClient did not close the connection, expected it to be rejected")
	}
}

func TestAddClientAuthDisabled(t *testing.T) {
	api := New() // no auth, no history
	defer api.Close()

	conn, _ := newHandshakeConn(t, "dev", "Mina", "")
	api.AddClient(conn)
	assertNotClosed(t, conn)
}

func TestAddClientValidToken(t *testing.T) {
	secret := []byte("test-secret")
	api := NewWithAuth(nil, secret)
	defer api.Close()

	token := signToken(t, secret, "Mina", []string{"dev", "ops"}, time.Hour)
	conn, _ := newHandshakeConn(t, "dev", "Mina", token)
	api.AddClient(conn)
	assertNotClosed(t, conn)
}

func TestAddClientMissingToken(t *testing.T) {
	api := NewWithAuth(nil, []byte("test-secret"))
	defer api.Close()

	conn, _ := newHandshakeConn(t, "dev", "Mina", "")
	api.AddClient(conn)
	assertClosed(t, conn)
}

func TestAddClientNameDoesNotMatchSubject(t *testing.T) {
	secret := []byte("test-secret")
	api := NewWithAuth(nil, secret)
	defer api.Close()

	token := signToken(t, secret, "SomeoneElse", []string{"dev"}, time.Hour)
	conn, _ := newHandshakeConn(t, "dev", "Mina", token)
	api.AddClient(conn)
	assertClosed(t, conn)
}

func TestAddClientRoomNotInClaim(t *testing.T) {
	secret := []byte("test-secret")
	api := NewWithAuth(nil, secret)
	defer api.Close()

	token := signToken(t, secret, "Mina", []string{"ops"}, time.Hour)
	conn, _ := newHandshakeConn(t, "dev", "Mina", token)
	api.AddClient(conn)
	assertClosed(t, conn)
}

func TestAddClientWrongSecret(t *testing.T) {
	api := NewWithAuth(nil, []byte("server-secret"))
	defer api.Close()

	token := signToken(t, []byte("other-secret"), "Mina", []string{"dev"}, time.Hour)
	conn, _ := newHandshakeConn(t, "dev", "Mina", token)
	api.AddClient(conn)
	assertClosed(t, conn)
}
