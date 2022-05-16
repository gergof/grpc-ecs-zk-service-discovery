package ZookeeperServiceDiscovery

import (
	"google.golang.org/grpc/resolver"
	"sync"
)

type zkResolverBuilder struct {
	scheme string
	zk     *zkClient
}

type zkResolver struct {
	zk          *zkClient
	path        string
	clientConn  resolver.ClientConn
	cancelWatch func()
	wg          sync.WaitGroup
}

func (b *zkResolverBuilder) Build(target resolver.Target, clientConn resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &zkResolver{
		zk:         b.zk,
		clientConn: clientConn,
		path:       target.URL.Path,
	}

	err := r.startWatch()

	if err != nil {
		return nil, err
	}

	return r, nil
}

func (b *zkResolverBuilder) Scheme() string {
	return b.scheme
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
