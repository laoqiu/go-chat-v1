package chat

import (
	"errors"

	"github.com/go-redis/redis"
)

type Queue struct {
	ChatKeyPrefix string
	AppKeyPrefix  string
	opts          *redis.Options
}

func newQueue(chatKey, appKey string, opts *redis.Options) *Queue {
	return &Queue{
		ChatKeyPrefix: chatKey,
		AppKeyPrefix:  appKey,
		opts:          opts,
	}
}

func (q *Queue) GetKey(project, dest string) string {
	return q.ChatKeyPrefix + project + ":" + dest
}

// Save 保存信息到消息队列
func (q *Queue) Save(project, dest, message string) error {
	c := redis.NewClient(q.opts)
	defer c.Close()
	key := q.GetKey(project, dest)
	if _, err := c.RPush(key, message).Result(); err != nil {
		return err
	}
	return nil
}

// Remove 删除一条信息
func (q *Queue) Remove(project, dest, message string) (int64, error) {
	c := redis.NewClient(q.opts)
	defer c.Close()
	key := q.GetKey(project, dest)
	return c.LRem(key, 0, message).Result()
}

// GetUnreadList 获得用户所有未读记录
func (q *Queue) GetUnreadList(project, dest string) ([]string, error) {
	c := redis.NewClient(q.opts)
	defer c.Close()
	key := q.GetKey(project, dest)
	return c.LRange(key, 0, -1).Result()
}

// GetAppInfo 查询app信息
func (q *Queue) GetAppInfo(key string) (map[string]string, error) {
	c := redis.NewClient(q.opts)
	defer c.Close()
	exist, err := c.Exists(q.AppKeyPrefix + key).Result()
	if err != nil {
		return nil, err
	}
	if exist == 0 {
		return nil, errors.New("不存在的appkey")
	}
	return c.HGetAll(q.AppKeyPrefix + key).Result()
}
