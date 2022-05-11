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
	conn     *zk.Conn
	canceler map[string]context.CancelFunc
}

func NewZkClient(servers []string, timeout int) (*zkClient, error) {
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
		conn:     conn,
		canceler: make(map[string]context.CancelFunc),
	}
	return client, nil
}

func (client *zkClient) createNode(path string, ip string) error {
	znodes := strings.Split(path, "/")

	var curPath string
	for i, znode := range znodes {
		if len(znode) == 0 {
			continue
		}

		curPath = curPath + "/" + znode
		exists, _, _ := client.conn.Exists(curPath)

		if exists {
			continue
		}

		if i != len(znodes)-1 {
			_, err := client.conn.Create(curPath, nil, 0, zk.WorldACL(zk.PermAll))
			if err != nil {
				grpclog.Infof("Failed to register node: %v", err)
				return err
			}
		} else {
			_, err := client.conn.Create(curPath, []byte(ip), zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
			if err != nil {
				grpclog.Infof("Failed to register node: %v", err)
				return err
			}
		}
	}

	grpclog.Infof("Registered client on path")

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
	client.Unlock()

	client.keepalive(ctx, path, ip)

	return nil
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
	client.Lock()
	for _, cancel := range client.canceler {
		cancel()
	}
	client.Unlock()
	client.conn.Close()
	client.conn = nil
}
