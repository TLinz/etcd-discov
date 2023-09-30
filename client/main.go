package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	greet_v1 "github.com/TLinz/etcd-discov/api/v1"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
)

const Scheme = "etcd"

var (
	ServiceName = flag.String("ServiceName", "greet_service", "service name")
	EtcdAddr    = flag.String("EtcdAddr", "127.0.0.1:2379", "register etcd address")
)

var cli *clientv3.Client

type etcdResolver struct {
	etcdAddr   string
	clientConn resolver.ClientConn
}

func newResolver(etcdAddr string) resolver.Builder {
	return &etcdResolver{etcdAddr: etcdAddr}
}

func (r *etcdResolver) Scheme() string {
	return Scheme
}

func (r *etcdResolver) ResolveNow(rn resolver.ResolveNowOptions) {
	log.Println("ResolveNow")
	fmt.Println(rn)
}

func (r *etcdResolver) Close() {
	log.Println("Close")
}

func (r *etcdResolver) Build(target resolver.Target, clientConn resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	var err error

	if cli == nil {
		cli, err = clientv3.New(clientv3.Config{
			Endpoints:   strings.Split(r.etcdAddr, ";"),
			DialTimeout: 15 * time.Second,
		})
		if err != nil {
			fmt.Printf("build etcd client err: %s\n", err)
			return nil, err
		}
	}

	r.clientConn = clientConn

	// go r.watch("/" + target.URL.Scheme + "/" + target.Endpoint() + "/")
	go r.watch("/" + target.Scheme + "/" + target.Endpoint + "/")

	return r, nil
}

func (r *etcdResolver) watch(keyPrefix string) {
	var addrList []resolver.Address

	resp, err := cli.Get(context.Background(), keyPrefix, clientv3.WithPrefix())
	if err != nil {
		fmt.Printf("get services addr lists err: %s\n", err)
	} else {
		for i := range resp.Kvs {
			addrList = append(addrList, resolver.Address{Addr: strings.TrimPrefix(string(resp.Kvs[i].Key), keyPrefix)})
		}
	}

	r.clientConn.UpdateState(resolver.State{Addresses: addrList})

	rch := cli.Watch(context.Background(), keyPrefix, clientv3.WithPrefix())
	for n := range rch {
		for _, ev := range n.Events {
			addr := strings.TrimPrefix(string(ev.Kv.Key), keyPrefix)
			// todo: use ev.Kv.Value directly
			switch ev.Type {
			case 0: // PUT
				if !exists(addrList, addr) {
					addrList = append(addrList, resolver.Address{Addr: addr})
					r.clientConn.UpdateState(resolver.State{Addresses: addrList})
				}
			case 1: // DELETE
				if s, ok := remove(addrList, addr); ok {
					addrList = s
					r.clientConn.UpdateState(resolver.State{Addresses: addrList})
				}
			}
		}
	}
}

func exists(l []resolver.Address, addr string) bool {
	for i := range l {
		if l[i].Addr == addr {
			return true
		}
	}
	return false
}

func remove(s []resolver.Address, addr string) ([]resolver.Address, bool) {
	for i := range s {
		if s[i].Addr == addr {
			s[i] = s[len(s)-1]
			return s[:len(s)-1], true
		}
	}
	return nil, false
}

func main() {
	flag.Parse()

	r := newResolver(*EtcdAddr)
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
