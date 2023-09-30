package main

import (
	"context"
	"flag"
	"log"
	"net"
	"time"

	greet_v1 "github.com/TLinz/etcd-discov/api/v1"
	"github.com/TLinz/etcd-discov/loadbalance"
	tinyBalancer "github.com/zehuamama/balancer/balancer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/resolver"
)

var (
	ServiceName = flag.String("ServiceName", "greet_service", "service name")
	EtcdAddr    = flag.String("EtcdAddr", "127.0.0.1:2379", "register etcd address")
)

// Get preferred outbound ip of this machine
func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func main() {
	flag.Parse()

	r, err := loadbalance.NewResolver(*EtcdAddr)
	if err != nil {
		log.Printf("init resolver err: %s", err)
		return
	}
	resolver.Register(r)

	// IPHashBalancer         = "ip-hash"
	// ConsistentHashBalancer = "consistent-hash"
	// P2CBalancer            = "p2c"
	// RandomBalancer         = "random"
	// R2Balancer             = "round-robin"
	// LeastLoadBalancer      = "least-load"
	// BoundedBalancer        = "bounded"
	bType := tinyBalancer.P2CBalancer
	p, err := loadbalance.NewPicker(bType, getOutboundIP().String())
	if err != nil {
		log.Printf("init balancer err: %s", err)
		return
	}
	balancer.Register(base.NewBalancerBuilder(bType, p, base.Config{}))

	// const grpcServiceConfig = `{"loadBalancingPolicy":"round_robin"}`
	// conn, err := grpc.Dial(r.Scheme()+"://author/"+*ServiceName,
	// 	grpc.WithDefaultServiceConfig(grpcServiceConfig), grpc.WithInsecure())

	conn, err := grpc.Dial(r.Scheme()+"://author/"+*ServiceName, grpc.WithBalancerName(bType), grpc.WithInsecure())
	if err != nil {
		log.Printf("connect srv err: %s\n", err)
	}
	defer conn.Close()

	c := greet_v1.NewGreetClient(conn)
	ticker := time.NewTicker(3 * time.Second)
	for range ticker.C {
		log.Println("Client calls Morning...")
		resp1, err := c.Morning(
			context.Background(),
			&greet_v1.GreetRequest{Name: "Lin"},
		)
		if err != nil {
			log.Printf("Client called Morning err: %s\n", err)
			return
		}
		log.Printf("Morning: %s, from: %s\n", resp1.Message, resp1.From)

		log.Println("Client calls Night...")
		resp2, err := c.Night(
			context.Background(),
			&greet_v1.GreetRequest{Name: "Lin"},
		)
		if err != nil {
			log.Printf("Client called Night err: %s\n", err)
			return
		}
		log.Printf("Night: %s, from: %s\n", resp2.Message, resp2.From)
	}
}
