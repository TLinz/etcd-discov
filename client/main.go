package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	greet_v1 "github.com/TLinz/etcd-discov/api/v1"
	"github.com/TLinz/etcd-discov/loadbalance"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
)

var (
	ServiceName = flag.String("ServiceName", "greet_service", "service name")
	EtcdAddr    = flag.String("EtcdAddr", "127.0.0.1:2379", "register etcd address")
)

func main() {
	flag.Parse()

	r, err := loadbalance.NewResolver(*EtcdAddr)
	if err != nil {
		log.Printf("init resolver err: %s", err)
		return
	}

	resolver.Register(r)

	const grpcServiceConfig = `{"loadBalancingPolicy":"round_robin"}`
	// conn, err := grpc.Dial(r.Scheme()+"://author/"+*ServiceName, grpc.WithDefaultServiceConfig(grpcServiceConfig),
	// grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc.Dial(r.Scheme()+"://author/"+*ServiceName,
		grpc.WithDefaultServiceConfig(grpcServiceConfig), grpc.WithInsecure())
	if err != nil {
		fmt.Printf("connect srv err: %s\n", err)
	}
	defer conn.Close()

	c := greet_v1.NewGreetClient(conn)
	ticker := time.NewTicker(3 * time.Second)
	for range ticker.C {
		fmt.Println("Client calls Morning...")
		resp1, err := c.Morning(
			context.Background(),
			&greet_v1.GreetRequest{Name: "Lin"},
		)
		if err != nil {
			fmt.Printf("Client called Morning err: %s\n", err)
			return
		}
		fmt.Printf("Morning: %s, from: %s\n", resp1.Message, resp1.From)

		fmt.Println("Client calls Night...")
		resp2, err := c.Night(
			context.Background(),
			&greet_v1.GreetRequest{Name: "Lin"},
		)
		if err != nil {
			fmt.Printf("Client called Night err: %s\n", err)
			return
		}
		fmt.Printf("Night: %s, from: %s\n", resp2.Message, resp2.From)
	}
}
