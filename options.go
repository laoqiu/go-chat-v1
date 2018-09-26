package chat

import (
	"github.com/go-redis/redis"
	"github.com/micro/go-micro/registry"
)

type Options struct {
	Name            string
	Version         string
	Topic           string
	RedisOptions    *redis.Options
	RegistryOptions []registry.Option
}

type Option func(o *Options)

func newOptions(opts ...Option) Options {
	opt := Options{
		Name:    DefaultName,
		Version: DefaultVersion,
		Topic:   DefaultTopic,
	}

	for _, o := range opts {
		o(&opt)
	}

	return opt
}

// Server name
func Name(n string) Option {
	return func(o *Options) {
		o.Name = n
	}
}

// Version of the service
func Version(v string) Option {
	return func(o *Options) {
		o.Version = v
	}
}

func Topic(v string) Option {
	return func(o *Options) {
		o.Topic = v
	}
}

func RegistryOptions(opts ...registry.Option) Option {
	return func(o *Options) {
		o.RegistryOptions = opts
	}
}

func RedisOptions(opts redis.Options) Option {
	return func(o *Options) {
		o.RedisOptions = &opts
	}
}
