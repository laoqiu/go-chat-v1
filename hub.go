package chat

import (
	"encoding/json"
	"log"
	"time"
)

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// 消息(点对点)
	message chan []byte

	// 注册client
	register chan *Client

	// 注销client
	unregister chan *Client
}

// NewHub get hub object
func NewHub() *Hub {
	return &Hub{
		message:    make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

// Run hub.Run
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			log.Println("register")
			h.clients[client] = true
		case client := <-h.unregister:
			log.Println("unregister")
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.message:
			// 解构消息
			m := &Message{}
			if err := json.Unmarshal(message, &m); err != nil {
				log.Println("Message unmarshal fail", err)
				continue
			}
			// TODO: 找出接收者
			for client := range h.clients {
				// 不符合用户跳过
				if client.project != m.Project || client.username != m.To {
					continue
				}
				select {
				case client.send <- message:
					log.Println("send to client")
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// Clean 清理不用的连接
func (h *Hub) Clean() {
	ticker := time.NewTicker(time.Second * 120)
	defer ticker.Stop()

	for {

		for c := range h.clients {
			// 判断是否有效连接
			if !c.isValid && time.Now().Sub(c.time).Seconds() > 30 {
				c.conn.Close()
			}
		}

		<-ticker.C
	}
}
