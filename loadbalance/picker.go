package loadbalance

import (
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"

	tinyBalancer "github.com/zehuamama/balancer/balancer"
)

var _ base.PickerBuilder = (*Picker)(nil)

type Picker struct {
	mu sync.Mutex

	algorithm  string
	clientIP   string
	tb         tinyBalancer.Balancer
	subconnMap map[string]balancer.SubConn
}

func NewPicker(algorithm, clientIP string) (*Picker, error) {
	p := &Picker{
		clientIP:   clientIP,
		algorithm:  algorithm,
		subconnMap: make(map[string]balancer.SubConn),
	}
	return p, nil
}

var _ balancer.Picker = (*Picker)(nil)

func (p *Picker) Build(buildInfo base.PickerBuildInfo) balancer.Picker {
	p.mu.Lock()
	defer p.mu.Unlock()
	var addrList []string
	for sc, scInfo := range buildInfo.ReadySCs {
		addr := scInfo.Address.Addr
		p.subconnMap[addr] = sc
		addrList = append(addrList, addr)
	}
	tb, err := tinyBalancer.Build(p.algorithm, addrList)
	if err != nil {
		return nil
	}
	p.tb = tb
	return p
}

func (p *Picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var result balancer.PickResult
	pickAddr, err := p.tb.Balance(p.clientIP)
	if err != nil {
		return result, err
	}
	result.SubConn = p.subconnMap[pickAddr]

	p.tb.Inc(pickAddr)
	defer p.tb.Done(pickAddr)

	return result, nil
}
