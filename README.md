# go-chat
基于go-micro的聊天组件

## 安装
go get github.com/laoqiu/go-chat

## 使用
main.go

```
func main() {
	// chat opts
	redisOpts := redis.Options{}
	regOpts := []registry.Option{}

	// create new web service
	service := web.NewService(
		web.Name("com.laoqiu.web.chat"),
		web.Version("latest"),
		web.Flags(
			cli.StringFlag{
				Name:   "redis_host",
				EnvVar: "REDIS_HOST",
				Usage:  "The redis_host e.g 127.0.0.1:6379",
			},
			cli.StringFlag{
				Name:   "redis_password",
				EnvVar: "REDIS_PASSWORD",
			},
		),
		web.Action(func(c *cli.Context) {
			if len(c.String("redis_host")) > 0 {
				redisOpts.Addr = c.String("redis_host")
			}
			if len(c.String("redis_password")) > 0 {
				redisOpts.Password = c.String("redis_password")
			}
			if len(c.String("registry_address")) > 0 {
				regOpts = append(regOpts, registry.Addrs(c.String("registry_address")))
			}
		}),
	)

	// initialise service
	if err := service.Init(); err != nil {
		log.Fatal(err)
	}

	// chat service
	cs := chat.NewService()
	cs.Init(
		chat.Name("chat"),
		chat.Version("v1"),
		chat.Topic("com.laoqiu.web.chat"),
		chat.RedisOptions(redisOpts),
		chat.RegistryOptions(regOpts...),
	)
	if err := cs.Run(); err != nil {
		log.Fatal(err)
	}

	// register html handler
	// service.Handle("/", http.FileServer(http.Dir("html")))

	// register chat handler
	service.HandleFunc("/stream", cs.NewHandler())

	// run service
	if err := service.Run(); err != nil {
		log.Fatal(err)
	}
}
```

## 消息格式及登录模式
TODO...
