package ZookeeperServiceDiscovery

import (
	"context"
	"github.com/go-zookeeper/zk"
	"github.com/matoous/go-nanoid/v2"
	"google.golang.org/grpc/grpclog"
	"strings"
	"sync"
	"time"
)

type zkClient struct {
	sync.RWMutex
	conn       *zk.Conn
	canceler   map[string]context.CancelFunc
	registered map[string]string
}

func newZkClient(servers []string, timeout int) (*zkClient, error) {
	if timeout == 0 {
		timeout = 5
	}
	conn, _, err := zk.Connect(servers, time.Second*time.Duration(timeout))

	if err != nil {
		grpclog.Errorf("Could not create ZooKeeper connection: %v", err)
		return nil, err
	}

	grpclog.Infof("Connected to ZooKeeper client")

	client := &zkClient{
		conn:       conn,
		canceler:   make(map[string]context.CancelFunc),
		registered: make(map[string]string),
	}
	return client, nil
}

func (client *zkClient) CreatePath(path string) (string, error) {
	znodes := strings.Split(path, "/")

	var curPath string
	for _, znode := range znodes {
		if len(znode) == 0 {
			continue
		}

		curPath = curPath + "/" + znode
		exists, _, _ := client.conn.Exists(curPath)

		if exists {
			continue
		}

		_, err := client.conn.Create(curPath, nil, 0, zk.WorldACL(zk.PermAll))
		if err != nil {
			grpclog.Infof("Failed to create znode: %v", err)
			return "", err
		}
	}

	return curPath, nil
}

func (client *zkClient) createNode(path string, ip string) error {
	znodes := strings.Split(path, "/")
	parentPath := strings.Join(znodes[:len(znodes)-1], "/")

	parent, err := client.CreatePath(parentPath)
	if err != nil {
		return err
	}

	clientPath := parent + "/" + znodes[len(znodes)-1]

	_, err = client.conn.Create(clientPath, []byte(ip), zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
	if err != nil {
		grpclog.Infof("Failed to register node: %v", err)
		return err
	}

	grpclog.Infof("Registered client on path: %v with IP: %v", clientPath, ip)
	return nil
}

func (client *zkClient) RegisterNode(path string, ip string) error {
	nodeId, err := gonanoid.New()

	if err != nil {
		return err
	}

	path = path + "/" + nodeId

	err = client.createNode(path, ip)

	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	client.Lock()
	client.canceler[nodeId] = cancel
	client.registered[ip] = nodeId
	client.Unlock()

	client.keepalive(ctx, path, ip)

	return nil
}

func (client *zkClient) GetRegisteredIps(path string) ([]string, error) {
	_, err := client.CreatePath(path)

	if err != nil {
		grpclog.Errorf("Failed to get nodes: %v", err)
		return nil, err
	}

	children, _, err := client.conn.Children(path)

	if err != nil {
		grpclog.Errorf("Failed to get nodes: %v", err)
		return nil, err
	}

	var ips []string
	for _, node := range children {
		val, _, err := client.conn.Get(path + "/" + node)

		if err != nil {
			grpclog.Errorf("Failed to get node value: %v", err)
			return nil, err
		}

		if client.registered[string(val)] != "" {
			// self registered, should skip
			continue
		}

		ips = append(ips, string(val))
	}

	return ips, nil
}

func (client *zkClient) WatchRegisteredIps(path string) (chan []string, func(), error) {
	_, err := client.CreatePath(path)

	if err != nil {
		grpclog.Errorf("Failed to set watch: %v", err)
		return nil, nil, err
	}

	ips := make(chan []string)

	_, _, evChan, err := client.conn.ChildrenW(path)

	if err != nil {
		grpclog.Errorf("Failed to set watch: %v", err)
		return nil, nil, err
	}

	id, err := gonanoid.New()

	if err != nil {
		grpclog.Errorf("Failed to set watch: %v", err)
		return nil, nil, err
	}

	watchId := "watch-" + id
	ctx, cancel := context.WithCancel(context.Background())
	client.Lock()
	client.canceler[watchId] = cancel
	client.Unlock()

	cancelFunc := func() {
		client.Lock()
		cancel()
		delete(client.canceler, watchId)
		client.Unlock()
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				ips <- nil
				return
			case ev := <-evChan:
				if ev.Type != zk.EventNodeChildrenChanged {
					_, _, evChan, _ = client.conn.ChildrenW(path)
					continue
				}

				curIps, _ := client.GetRegisteredIps(path)
				ips <- curIps
			}
		}
	}()

	return ips, cancelFunc, nil
}

func (client *zkClient) keepalive(ctx context.Context, path string, ip string) {
	ticker := time.NewTicker(time.Second)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if client.conn.State() != zk.StateHasSession {
					client.createNode(path, ip)
				}
			}
		}
	}()
}

func (client *zkClient) Close() {
	client.RLock()
	for _, cancel := range client.canceler {
		cancel()
	}
	client.RUnlock()
	client.conn.Close()
	client.conn = nil
}
