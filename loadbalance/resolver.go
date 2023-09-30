package loadbalance

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
)

const Scheme = "etcd"

// var cli *clientv3.Client

type etcdResolver struct {
	etcdAddr   string
	clientConn resolver.ClientConn
	cli        *clientv3.Client
}

func NewResolver(etcdAddr string) (resolver.Builder, error) {
	r := &etcdResolver{etcdAddr: etcdAddr}

	var err error
	if r.cli == nil {
		r.cli, err = clientv3.New(clientv3.Config{
			Endpoints:   strings.Split(r.etcdAddr, ";"),
			DialTimeout: 15 * time.Second,
		})
		if err != nil {
			fmt.Printf("connect to etcd err: %s\n", err)
			return nil, err
		}
	}
	return r, nil
}

func (r *etcdResolver) Scheme() string {
	return Scheme
}

func (r *etcdResolver) ResolveNow(rn resolver.ResolveNowOptions) {
	log.Printf("resolve now, options: %v\n", rn)
}

func (r *etcdResolver) Close() {
	log.Println("Close")
}

func (r *etcdResolver) Build(target resolver.Target, clientConn resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {

	r.clientConn = clientConn

	// go r.watch("/" + target.URL.Scheme + "/" + target.Endpoint() + "/")
	go r.watch("/" + target.Scheme + "/" + target.Endpoint + "/")

	return r, nil
}

func (r *etcdResolver) watch(keyPrefix string) {
	var addrList []resolver.Address

	resp, err := r.cli.Get(context.Background(), keyPrefix, clientv3.WithPrefix())
	if err != nil {
		fmt.Printf("get services addr lists err: %s\n", err)
	} else {
		for i := range resp.Kvs {
			addrList = append(addrList, resolver.Address{Addr: strings.TrimPrefix(string(resp.Kvs[i].Key), keyPrefix)})
		}
	}

	r.clientConn.UpdateState(resolver.State{Addresses: addrList})

	rch := r.cli.Watch(context.Background(), keyPrefix, clientv3.WithPrefix())
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
