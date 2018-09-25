package chat

import (
	"encoding/json"
	"log"

	"github.com/micro/go-micro/broker"
)

type Broker struct {
	topic  string
	client broker.Broker
	queue  *Queue
}

func NewBroker(topic string, queue *Queue, opts ...broker.Option) *Broker {
	return &Broker{
		topic:  topic,
		queue:  queue,
		client: broker.NewBroker(opts...),
	}
}

func (b *Broker) Init() error {
	if err := b.client.Init(); err != nil {
		return err
	}
	if err := b.client.Connect(); err != nil {
		return err
	}
	return nil
}

func (b *Broker) Publish(project string, m *Message) error {
	message, err := json.Marshal(m)
	if err != nil {
		return err
	}
	// 推送前需要同步保存到redis
	if err := b.queue.Save(project, m.To, string(message)); err != nil {
		return err
	}
	msg := &broker.Message{
		Body: message,
	}
	if err := b.client.Publish(b.topic, msg); err != nil {
		return err
	}
	return nil
}

func (b *Broker) Subscribe(hub *Hub) {
	if _, err := b.client.Subscribe(b.topic, func(p broker.Publication) error {
		log.Println("[sub] received message:", string(p.Message().Body), "header", p.Message().Header)
		hub.message <- p.Message().Body
		return nil
	}); err != nil {
		log.Println("[sub] fail", err)
	}
}
