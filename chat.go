package chat

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/micro/go-micro/broker"
	"github.com/micro/go-micro/registry"
)

var (
	DefaultName    = "go-chat"
	DefaultVersion = "latest"
	DefaultTopic   = "go.micro.web.chat"
	upgrader       = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type service struct {
	hub    *Hub
	broker *Broker
}

func NewService() *service {
	return &service{
		hub: NewHub(),
	}
}

func (s *service) Init(opts ...Option) error {
	o := newOptions(opts...)
	// key
	chatKeyPrefix := fmt.Sprintf("%s:%s:", o.Name, o.Version)
	appKeyPrefix := chatKeyPrefix + "app:"
	// broker
	s.broker = NewBroker(
		o.Topic,
		newQueue(chatKeyPrefix, appKeyPrefix, o.RedisOptions),
		broker.Registry(
			registry.NewRegistry(o.RegistryOptions...),
		),
	)
	return nil
}

func (s *service) Run() error {
	if err := s.broker.Init(); err != nil {
		return err
	}
	go s.hub.Run()
	go s.hub.Clean()
	go s.broker.Subscribe(s.hub)
	return nil
}

// NewHandler handles websocket requests from the peer.
func (s *service) NewHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()
	client := &Client{broker: s.broker, hub: s.hub, conn: conn, send: make(chan []byte, 256)}
	client.time = time.Now()
	client.hub.register <- client
	go client.writer()
	client.reader()
}
