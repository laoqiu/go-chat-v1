package chat

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 50 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 8 * 1024
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

// Message 消息结构体 TODO: 支持群聊
type Message struct {
	ID string `json:"id"` // 客户端唯一编码
	//ChatType string `json:"chatType"` // 单聊、群聊(singleChat、groupChat)
	//GroupID  string `json:"groupID"`  // 群组(群、公聊、聊天室)，ChatType为groupChat时有效
	Type    string `json:"type"`    // 消息类型
	From    string `json:"from"`    // 来自
	To      string `json:"to"`      // 去向
	Content string `json:"content"` // 内容
	Project string `json:"project"` // 项目ID，用于实现多项目通用
}

// 登录验证客户端
type ValidMessage struct {
	ID       string `json:"id"`       // 客户端唯一编码
	APPKey   string `json:"appKey"`   // app key
	Username string `json:"username"` // username
	Token    string `json:"token"`    // token
}

// 客户端
type Client struct {
	broker *Broker

	hub *Hub

	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// 是否已验证客户端
	isValid bool

	// 项目id
	project string

	// 已登录用户
	username string

	// 连接时间
	time time.Time
}

func (c *Client) reader() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		// 读取客户端消息
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("ReadMessageError: %v", err)
			// 判断是否异常断开
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("IsUnexpectedCloseError: %v", err)
			}
			break
		}

		// reader
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		log.Println("Reader message: ", string(message))

		// 未验证强制验证
		if !c.isValid {
			vm := &ValidMessage{}
			if err := json.Unmarshal(message, vm); err != nil {
				// 验证失败
				log.Println("验证失败:", err)
				break
			}
			// 登录
			if err := c.login(vm); err != nil {
				// 登录失败
				log.Println("登录失败:", err)
				break
			}
			continue
		}

		// 解构消息
		m := &Message{}
		if err := json.Unmarshal(message, &m); err != nil {
			// 消息有问题忽略掉
			continue
		}

		switch m.Type {
		case "message", "read":
			if len(m.To) == 0 {
				c.writeError(m.ID, "缺少参数to")
			} else {
				// 发送消息
				if err := c.sendBroker(m); err != nil {
					c.writeError(m.ID, "消息发送失败")
				}
			}
		default:
			c.writeError(m.ID, "不支持的消息类型")
		}
	}
}

func (c *Client) writer() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			//w.Write(message)
			if err := c._writeMessage(w, message); err != nil {
				log.Println("client write message fail ->", err)
			}

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				//w.Write(<-c.send)
				if err := c._writeMessage(w, <-c.send); err != nil {
					log.Println("client write message fail ->", err)
				}
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// 检查token有效性接口
func (c *Client) checkToken(api, username, token string) error {
	match, err := regexp.MatchString(`^(https?):\/\/.+$`, api)
	if err != nil {
		return err
	}
	if !match {
		return errors.New("无效API地址")
	}
	data, _ := json.Marshal(map[string]string{
		"username": username,
		"token":    token,
	})
	rsp, err := http.Post(api, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	body, _ := ioutil.ReadAll(rsp.Body)
	log.Println(string(body))

	type Result struct {
		Code string `json:"code"`
	}
	result := &Result{}
	if err := json.Unmarshal(body, result); err != nil {
		return err
	}
	if result.Code != "0" {
		return errors.New("token认证失败")
	}
	return nil
}

func (c *Client) login(vm *ValidMessage) error {
	// 查询app信息，验证appkey有效性
	info, err := c.broker.queue.GetAppInfo(vm.APPKey)
	if err != nil {
		return err
	}

	// 验证token
	if err := c.checkToken(info["api"], vm.Username, vm.Token); err != nil {
		return err
	}

	// 登录成功 (最好是服务端返回用户id)
	c.username = vm.Username
	c.project = info["project_id"]
	c.isValid = true

	log.Println("Login Success")

	// 返回登录成功回执
	if err := c.writeReceipt(vm.ID, vm.Username, "logged"); err != nil {
		return err
	}

	// 返回用户所有未读消息
	result, err := c.broker.queue.GetUnreadList(c.project, c.username)
	if err != nil {
		return err
	}

	for _, v := range result {
		select {
		case c.send <- []byte(v):
			log.Println("send to client")
		default:
			close(c.send)
			delete(c.hub.clients, c)
		}
	}
	return nil
}

// 分发到所有服务器
func (c *Client) sendBroker(m *Message) error {
	// 加上必要信息
	m.From = c.username
	m.Project = c.project

	// broker publish 分发消息给所有监听的微服务
	if err := c.broker.Publish(c.project, m); err != nil {
		return err
	}

	// 如果是普通消息已发送回执
	if m.Type == "message" {
		if err := c.writeReceipt(m.ID, m.From, "received"); err != nil {
			log.Println("已发送回执消息发送失败", m.ID)
		}
	}
	return nil
}

func (c *Client) _writeMessage(w io.WriteCloser, message []byte) error {
	w.Write(message)
	log.Println("w.Write", string(message))
	// 解构消息
	m := &Message{}
	if err := json.Unmarshal(message, &m); err != nil {
		return err
	}
	// 消息需要从队列移除消息
	if _, err := c.broker.queue.Remove(c.project, m.To, string(message)); err != nil {
		return err
	}
	// 普通消息需要发送已送达回执
	if m.Type == "message" {
		if err := c.writeReceipt(m.ID, m.From, "delivered"); err != nil {
			log.Println("已送达回执消息发送失败", m.ID)
		}
	}
	return nil
}

// 返回错误信息
func (c *Client) writeError(id, errorMessage string) {
	message, err := json.Marshal(Message{
		ID:      id,
		To:      c.username,
		Type:    "error",
		Content: errorMessage,
	})
	if err != nil {
		log.Println("writeError error:", err)
	} else {
		c.send <- message
	}
}

// 返回回执信息
func (c *Client) writeReceipt(id, to, receiptType string) error {
	m := &Message{
		ID:      id,
		To:      to,
		Type:    receiptType,
		Project: c.project,
	}
	// 本客户端回执直接返回
	if c.username == to {
		message, err := json.Marshal(m)
		if err != nil {
			return err
		}
		c.send <- message
		return nil
	}

	// 分发回执到服务器
	if err := c.broker.Publish(c.project, m); err != nil {
		return err
	}
	return nil
}
