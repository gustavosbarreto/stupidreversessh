package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	gliderssh "github.com/gliderlabs/ssh"
	"github.com/gorilla/mux"
	"github.com/shellhub-io/shellhub/pkg/api/client"
)

type Agent struct {
	router *mux.Router
	server *http.Server
	sshd   *gliderssh.Server
	cli    client.Client
}

func (a *Agent) handler(client gliderssh.Session) {
	fmt.Println("handler")
	client.Close()
}

func main() {
	addr, _ := url.Parse("http://localhost:8080")

	agent := &Agent{
		router: mux.NewRouter(),
		server: &http.Server{
			ConnContext: func(ctx context.Context, c net.Conn) context.Context {
				return context.WithValue(ctx, "http-conn", c)
			},
		},
		sshd: &gliderssh.Server{
			PasswordHandler: func(ctx gliderssh.Context, password string) bool {
				return true
			},
			SessionRequestCallback: func(client gliderssh.Session, request string) bool {
				return true
			},
		},
		cli: client.NewClient(client.WithURL(addr)),
	}

	agent.router.HandleFunc("/ssh/{id}", func(w http.ResponseWriter, r *http.Request) {
		conn, ok := r.Context().Value("http-conn").(net.Conn)
		if !ok {
			panic("panico")
		}

		fmt.Println(conn.LocalAddr())
		fmt.Println("handle")

		agent.sshd.HandleConn(conn)

		select {}
		fmt.Println("oi")
	})

	agent.sshd.Handler = agent.handler
	agent.server.Handler = agent.router

	listener, err := agent.cli.NewReverseListener("")
	if err != nil {
		panic(err)
	}

	if err := agent.server.Serve(listener); err != nil {
		panic(err)
	}

	select {}
}
