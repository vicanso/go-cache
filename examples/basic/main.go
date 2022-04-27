package main

import (
	"context"
	"fmt"
	"time"

	"github.com/vicanso/go-cache/v2"
)

type Data struct {
	Name string `json:"name"`
}

func main() {
	// 默认使用big cache
	// big cache不支持每个key设置不同的缓存时长
	c, err := cache.New(10 * time.Second)
	if err != nil {
		fmt.Println(err)
	}
	defer c.Close(context.Background())
	key := "key"
	data := Data{
		Name: "tree.xie",
	}
	err = c.Set(context.Background(), key, &data)
	if err != nil {
		fmt.Println(err)
	}
	data = Data{}
	err = c.Get(context.Background(), key, &data)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(data)

	result, err := cache.Get[Data](context.Background(), c, key)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(result)
}
