package main

import (
	"fmt"
	"net/http"

	gliderssh "github.com/gliderlabs/ssh"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/shellhub-io/shellhub/pkg/connman"
	"github.com/shellhub-io/shellhub/pkg/revdial"
	"github.com/shellhub-io/shellhub/pkg/wsconnadapter"
	"golang.org/x/crypto/ssh"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	Subprotocols:    []string{"binary"},
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	router  *mux.Router
	connman *connman.ConnectionManager
	sshd    *gliderssh.Server
}

func (s *Server) handler(client gliderssh.Session) {
	conn, err := s.connman.Dial(client.Context(), "id")
	if err != nil {
		panic(err)
	}

	// SSH client to tunnel connection
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/ssh/%s", "id"), nil)
	if err = req.Write(conn); err != nil {
		panic(err)
	}

	config := &ssh.ClientConfig{
		User:            "gustavo",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.Password("password"),
		},
	}

	cli, chans, _, err := ssh.NewClientConn(conn, "tcp", config)
	if err != nil {
		fmt.Println(err)
		client.Close()
		conn.Close()
		return
	}

	ch := make(chan *ssh.Request)
	close(ch)

	ssh.NewClient(cli, chans, ch)
}

func main() {
	server := &Server{
		router:  mux.NewRouter(),
		connman: connman.New(),
		sshd: &gliderssh.Server{
			Addr: ":2222",
			PasswordHandler: func(ctx gliderssh.Context, password string) bool {
				fmt.Println("password accepted")
				return true
			},
			SessionRequestCallback: func(client gliderssh.Session, request string) bool {
				return true
			},
		},
	}

	// FIX nil pointer
	server.connman.DialerKeepAliveCallback = func(string, *revdial.Dialer) {
	}

	server.sshd.Handler = server.handler

	server.router.HandleFunc("/ssh/connection", func(res http.ResponseWriter, req *http.Request) {
		conn, err := upgrader.Upgrade(res, req, nil)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)

			return
		}

		server.connman.Set("id", wsconnadapter.New(conn))
	}).Methods(http.MethodGet)

	server.router.Handle("/ssh/revdial", revdial.ConnHandler(upgrader)).Methods(http.MethodGet)

	go http.ListenAndServe(":8080", server.router)

	go server.sshd.ListenAndServe()

	select {}
}
