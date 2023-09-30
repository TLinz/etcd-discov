package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	greet_v1 "github.com/TLinz/etcd-discov/api/v1"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

const Scheme = "etcd"

var (
	cli         *clientv3.Client
	Host        = "127.0.0.1"
	Port        = flag.Int("Port", 3000, "listening port")
	ServiceName = flag.String("ServiceName", "greet_service", "service name")
	EtcdAddr    = flag.String("EtcdAddr", "127.0.0.1:2379", "register etcd address")
)

type greetServer struct {
	greet_v1.UnimplementedGreetServer
}

func (gs *greetServer) Morning(ctx context.Context, req *greet_v1.GreetRequest) (*greet_v1.GreetResponse, error) {
	fmt.Printf("Morning! %s\n", req.Name)
	return &greet_v1.GreetResponse{
		Message: "Good morning, " + req.Name,
		From:    fmt.Sprintf("127.0.0.1:%d", *Port),
	}, nil
}

func (gs *greetServer) Night(ctx context.Context, req *greet_v1.GreetRequest) (*greet_v1.GreetResponse, error) {
	fmt.Printf("Night! %s\n", req.Name)
	return &greet_v1.GreetResponse{
		Message: "Good night, " + req.Name,
		From:    fmt.Sprintf("127.0.0.1:%d", *Port),
	}, nil
}

func register(etcdAddr, serviceName, serverAddr string, ttl int64) error {
	var err error

	if cli == nil {
		cli, err = clientv3.New(clientv3.Config{
			Endpoints:   strings.Split(etcdAddr, ";"),
			DialTimeout: 15 * time.Second,
		})
		if err != nil {
			fmt.Printf("build etcd client err: %s\n", err)
			return err
		}
	}

	key := "/" + Scheme + "/" + serviceName + "/" + serverAddr
	resp, err := cli.Get(context.Background(), key)
	if err != nil {
		fmt.Printf("get services addr lists err: %s\n", err)
	} else if resp.Count == 0 {
		err = keepAlive(serviceName, serverAddr, ttl)
		if err != nil {
			fmt.Printf("keep alive err: %s", err)
		}
	}

	return nil
}

func keepAlive(serviceName, serverAddr string, ttl int64) error {
	leaseResp, err := cli.Grant(context.Background(), ttl)
	if err != nil {
		fmt.Printf("create lease err: %s\n", err)
		return err
	}

	key := "/" + Scheme + "/" + serviceName + "/" + serverAddr
	_, err = cli.Put(context.Background(), key, serverAddr, clientv3.WithLease(leaseResp.ID))
	if err != nil {
		fmt.Printf("register addr err: %s\n", err)
		return err
	}

	ch, err := cli.KeepAlive(context.Background(), leaseResp.ID)
	if err != nil {
		fmt.Printf("set up keep-alive conn err: %s\n", err)
		return err
	}

	go func() {
		for {
			<-ch
		}
	}()
	return nil
}

func unRegister(serviceName, serverAddr string) {
	if cli != nil {
		key := "/" + Scheme + "/" + serviceName + "/" + serverAddr
		cli.Delete(context.Background(), key)
	}
	// todo: maybe better to cancel corresponding lease
}

func main() {
	flag.Parse()

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *Port))
	if err != nil {
		fmt.Println("net listen err: ", err)
		return
	}
	defer listener.Close()

	srv := grpc.NewServer()
	defer srv.GracefulStop()

	greet_v1.RegisterGreetServer(srv, &greetServer{})

	serverAddr := fmt.Sprintf("%s:%d", Host, *Port)
	fmt.Printf("greeting server address: %s\n", serverAddr)
	register(*EtcdAddr, *ServiceName, serverAddr, 5)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		s := <-ch
		unRegister(*ServiceName, serverAddr)
		if i, ok := s.(syscall.Signal); ok {
			os.Exit(int(i))
		} else {
			os.Exit(0)
		}
	}()

	err = srv.Serve(listener)
	if err != nil {
		fmt.Println("srv listen err: ", err)
		return
	}
}
