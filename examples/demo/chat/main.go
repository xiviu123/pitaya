package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"strings"

	"github.com/topfreegames/pitaya"
	"github.com/topfreegames/pitaya/acceptor"
	"github.com/topfreegames/pitaya/component"
	"github.com/topfreegames/pitaya/serialize/json"
	"github.com/topfreegames/pitaya/session"
)

type (
	// Room represents a component that contains a bundle of room related handler
	// like Join/Message
	Room struct {
		component.Base
		group *pitaya.Group
		timer *pitaya.Timer
		stats *stats
	}

	// UserMessage represents a message that user sent
	UserMessage struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}

	// NewUser message will be received when new user join room
	NewUser struct {
		Content string `json:"content"`
	}

	// AllMembers contains all members uid
	AllMembers struct {
		Members []int64 `json:"members"`
	}

	// JoinResponse represents the result of joining room
	JoinResponse struct {
		Code   int    `json:"code"`
		Result string `json:"result"`
	}

	stats struct {
		outboundBytes int
		inboundBytes  int
	}
)

func (stats *stats) outbound(s *session.Session, in []byte) ([]byte, error) {
	stats.outboundBytes += len(in)
	return in, nil
}

func (stats *stats) inbound(s *session.Session, in []byte) ([]byte, error) {
	stats.inboundBytes += len(in)
	return in, nil
}

// NewRoom returns a new room
func NewRoom() *Room {
	return &Room{
		group: pitaya.NewGroup("room"),
		stats: &stats{},
	}
}

// AfterInit component lifetime callback
func (r *Room) AfterInit() {
	r.timer = pitaya.NewTimer(time.Minute, func() {
		println("UserCount: Time=>", time.Now().String(), "Count=>", r.group.Count())
		println("OutboundBytes", r.stats.outboundBytes)
		println("InboundBytes", r.stats.outboundBytes)
	})
}

// Join room
func (r *Room) Join(s *session.Session, msg []byte) error {
	fakeUID := s.ID() //just use s.ID as uid !!!
	s.Bind(fakeUID)   // binding session uid

	s.Push("onMembers", &AllMembers{Members: r.group.Members()})
	// notify others
	r.group.Broadcast("onNewUser", &NewUser{Content: fmt.Sprintf("New user: %d", s.ID())})
	// new user join group
	r.group.Add(s) // add session to group

	// on session close, remove it from group
	s.OnClose(func() {
		r.group.Leave(s)
	})

	return s.Response(&JoinResponse{Result: "success"})
}

// Message sync last message to all members
func (r *Room) Message(s *session.Session, msg *UserMessage) error {
	return r.group.Broadcast("onMessage", msg)
}

func main() {
	defer (func() {
		pitaya.Shutdown()
	})()

	pitaya.SetSerializer(json.NewSerializer())

	// rewrite component and handler name
	room := NewRoom()
	pitaya.Register(room,
		component.WithName("room"),
		component.WithNameFunc(strings.ToLower),
	)

	// traffic stats
	pitaya.Pipeline.Outbound.PushBack(room.stats.outbound)
	pitaya.Pipeline.Inbound.PushBack(room.stats.inbound)

	log.SetFlags(log.LstdFlags | log.Llongfile)
	//TODO fix pitaya.SetWSPath("/pitaya")

	http.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.Dir("web"))))

	//TODO need to fix that? pitaya.SetCheckOriginFunc(func(_ *http.Request) bool { return true })
	ws := acceptor.NewWSAcceptor(":3250", "/pitaya")
	tcp := acceptor.NewTCPAcceptor(":3255")
	pitaya.AddAcceptor(ws)
	pitaya.AddAcceptor(tcp)
	pitaya.Start()
}
