package ZookeeperServiceDiscovery

import (
	"fmt"
	"google.golang.org/grpc/resolver"
	"net"
	"sync"
)

type zkResolver struct {
	scheme      string
	zk          *zkClient
	path        string
	clientConn  resolver.ClientConn
	cancelWatch func()
	wg          sync.WaitGroup
}

func (r *zkResolver) Build(target resolver.Target, clientConn resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r.clientConn = clientConn

	targetPath, _, err := net.SplitHostPort(target.URL.Path)

	if err != nil {
		return nil, fmt.Errorf("Failed to parse path: %v", err)
	}

	fmt.Printf("TARGET PATH: %v", targetPath)

	r.path = targetPath

	err = r.startWatch()

	if err != nil {
		return nil, err
	}

	return r, nil
}

func (r *zkResolver) startWatch() error {
	ipsChan, unsub, err := r.zk.WatchRegisteredIps(r.path)

	if err != nil {
		return err
	}

	r.cancelWatch = unsub

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for ips := range ipsChan {
			if ips == nil {
				// watch done
				break
			}

			addrs := []resolver.Address{}
			for _, ip := range ips {
				addrs = append(addrs, resolver.Address{Addr: ip})
			}

			r.clientConn.UpdateState(resolver.State{Addresses: addrs})
		}
	}()

	return nil
}

func (r *zkResolver) Scheme() string {
	return r.scheme
}

func (r *zkResolver) ResolveNow(opts resolver.ResolveNowOptions) {
	ips, err := r.zk.GetRegisteredIps(r.path)

	if err != nil {
		return
	}

	addrs := []resolver.Address{}
	for _, ip := range ips {
		addrs = append(addrs, resolver.Address{Addr: ip})
	}

	r.clientConn.UpdateState(resolver.State{Addresses: addrs})
}

func (r *zkResolver) Close() {
	if r.cancelWatch != nil {
		r.cancelWatch()
		r.cancelWatch = nil
	}

	r.wg.Wait()
}
