# grpc-ecs-zk-service-discovery

gRPC service discovery for Go using Apace ZooKeeper

### Usage

```go
package main

import (
	"fmt"
	"github.com/gergof/grpc-ecs-zk-service-discovery"
	"google.golang.org/grpc"
)

func main() {
	// Create zookeeper service discovery instance
	// You should pass all the zookeeper servers, so if one goes down, the service
	// will reconnect to another
	// The timeout should be a small value, 3-5 seconds should be plenty
	// You can choose any path you want. Remember, this path will only be used
	// to **register** the services to
	zkSd, err := ZookeeperServiceDiscovery.NewEcsZkServiceDiscovery(
		[]string{
			"zk-1.zookeeper.example.com",
			"zk-2.zookeeper.example.com",
			"zk-3.zookeeper.example.com",
		},
		5,
		"/com.example.some-service/service-discovery",
	)

	if err != nil {
		fmt.Errorf("Could not initialize service discovery: %w", err)
		return
	}

	// Register the gRPC resolver
	// You need to call this before calling grpc.Dial()
	zkSd.RegisterResolver()

	// Register a service with a port and an optional host
	err = zkSd.RegisterService("8080")

	// or:
	err = zkSd.RegisterService("8888", "10.0.1.5")

	if err != nil {
		fmt.Errorf("Could not register service: %w", err)
		return
	}

	// You should always clean up before exiting and before stopping the service itself
	// so the service will be unregistered before it stops to answer requests
	// Note: After calling unregister, you **can not** restart the service discovery, you
	// need to create a new instance
	defer zkSd.Unregister()

	// Use the zk:// scheme to dial a gRPC service
	c, _ := grpc.Dial("zk:///com.example.some-service/service-discovery", grpc.WithInsecure())
	defer c.Close()
}

```
