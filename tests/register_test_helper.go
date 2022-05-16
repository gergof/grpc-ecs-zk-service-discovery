package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gergof/grpc-ecs-zk-service-discovery"
	"github.com/go-zookeeper/zk"
)

func TestWithMetadataResponse(t *testing.T, metadata string) {
	srvECS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, metadata)
	}))
	srvEC2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "10.0.2.100")
	}))
	os.Setenv("ECS_CONTAINER_METADATA_URI_V4", srvECS.URL)
	os.Setenv("EC2_METADATA_IP", srvEC2.URL)
	defer srvECS.Close()
	defer srvEC2.Close()
	defer os.Unsetenv("ECS_CONTAINER_METADATA_URI_V4")
	defer os.Unsetenv("EC2_METADATA_IP")

	sd, err := ZookeeperServiceDiscovery.NewEcsZkServiceDiscovery([]string{"localhost:2181"}, 5, "_test/eu.systest.grpc-zk-sd/nodes")
	defer sd.Unregister()

	if err != nil {
		t.Errorf("expected err to be nil got %v", err)
		return
	}

	err = sd.RegisterService(41500)

	time.Sleep(50 * time.Millisecond)

	if err != nil {
		t.Errorf("expected err to be nil got %v", err)
		return
	}

	conn, _, _ := zk.Connect([]string{"localhost:2181"}, time.Second*time.Duration(5))

	children, _, _ := conn.Children("/_test/eu.systest.grpc-zk-sd/nodes")

	if len(children) != 1 {
		t.Errorf("expected number of ZK children to be 1 got %v", len(children))
		return
	}

	val, _, _ := conn.Get("/_test/eu.systest.grpc-zk-sd/nodes" + "/" + children[0])

	if string(val) != "10.0.2.100:41500" {
		t.Errorf("expected registered IP to be 10.0.2.100:41500 got %v", string(val))
		return
	}
}
