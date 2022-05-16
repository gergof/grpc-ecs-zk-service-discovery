package main

import (
	"net"
	"testing"
	"time"

	"github.com/gergof/grpc-ecs-zk-service-discovery"
	"github.com/go-zookeeper/zk"
)

func Test_RegisterWithVPC(t *testing.T) {
	TestWithMetadataResponse(t, `{"DockerId":"ea32192c8553fbff06c9340478a2ff089b2bb5646fb718b4ee206641c9086d66","Name":"curl","DockerName":"ecs-curltest-24-curl-cca48e8dcadd97805600","Image":"111122223333.dkr.ecr.us-west-2.amazonaws.com/curltest:latest","ImageID":"sha256:d691691e9652791a60114e67b365688d20d19940dde7c4736ea30e660d8d3553","Labels":{"com.amazonaws.ecs.cluster":"default","com.amazonaws.ecs.container-name":"curl","com.amazonaws.ecs.task-arn":"arn:aws:ecs:us-west-2:111122223333:task/default/8f03e41243824aea923aca126495f665","com.amazonaws.ecs.task-definition-family":"curltest","com.amazonaws.ecs.task-definition-version":"24"},"DesiredStatus":"RUNNING","KnownStatus":"RUNNING","Limits":{"CPU":10,"Memory":128},"CreatedAt":"2020-10-02T00:15:07.620912337Z","StartedAt":"2020-10-02T00:15:08.062559351Z","Type":"NORMAL","LogDriver":"awslogs","LogOptions":{"awslogs-create-group":"true","awslogs-group":"/ecs/metadata","awslogs-region":"us-west-2","awslogs-stream":"ecs/curl/8f03e41243824aea923aca126495f665"},"ContainerARN":"arn:aws:ecs:us-west-2:111122223333:container/0206b271-b33f-47ab-86c6-a0ba208a70a9","Networks":[{"NetworkMode":"awsvpc","IPv4Addresses":["10.0.2.100"],"AttachmentIndex":0,"MACAddress":"0e:9e:32:c7:48:85","IPv4SubnetCIDRBlock":"10.0.2.0/24","PrivateDNSName":"ip-10-0-2-100.us-west-2.compute.internal","SubnetGatewayIpv4Address":"10.0.2.1/24"}]}`)
}

func Test_RegisterWithBridge(t *testing.T) {
	TestWithMetadataResponse(t, `{"DockerId":"ea32192c8553fbff06c9340478a2ff089b2bb5646fb718b4ee206641c9086d66","Name":"curl","DockerName":"ecs-curltest-24-curl-cca48e8dcadd97805600","Image":"111122223333.dkr.ecr.us-west-2.amazonaws.com/curltest:latest","ImageID":"sha256:d691691e9652791a60114e67b365688d20d19940dde7c4736ea30e660d8d3553","Labels":{"com.amazonaws.ecs.cluster":"default","com.amazonaws.ecs.container-name":"curl","com.amazonaws.ecs.task-arn":"arn:aws:ecs:us-west-2:111122223333:task/default/8f03e41243824aea923aca126495f665","com.amazonaws.ecs.task-definition-family":"curltest","com.amazonaws.ecs.task-definition-version":"24"},"DesiredStatus":"RUNNING","KnownStatus":"RUNNING","Limits":{"CPU":10,"Memory":128},"CreatedAt":"2020-10-02T00:15:07.620912337Z","StartedAt":"2020-10-02T00:15:08.062559351Z","Type":"NORMAL","LogDriver":"awslogs","LogOptions":{"awslogs-create-group":"true","awslogs-group":"/ecs/metadata","awslogs-region":"us-west-2","awslogs-stream":"ecs/curl/8f03e41243824aea923aca126495f665"},"ContainerARN":"arn:aws:ecs:us-west-2:111122223333:container/0206b271-b33f-47ab-86c6-a0ba208a70a9","Networks":[{"NetworkMode":"bridge","IPv4Addresses":["172.17.0.1"]}]}`)
}

func Test_RegisterWithoutECS(t *testing.T) {
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

	var localIp string
	addrs, _ := net.InterfaceAddrs()

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() {
			localIp = ipnet.IP.String()
			break
		}
	}

	if string(val) != localIp+":41500" {
		t.Errorf("expected registered IP to be %s:41500 got %v", localIp, string(val))
		return
	}
}

func Test_RegisterWithManualIP(t *testing.T) {
	sd, err := ZookeeperServiceDiscovery.NewEcsZkServiceDiscovery([]string{"localhost:2181"}, 5, "_test/eu.systest.grpc-zk-sd/nodes")
	defer sd.Unregister()

	if err != nil {
		t.Errorf("expected err to be nil got %v", err)
		return
	}

	err = sd.RegisterService(41500, "192.0.2.10")

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

	if string(val) != "192.0.2.10:41500" {
		t.Errorf("expected registered IP to be 192.0.2.10:41500 got %v", string(val))
		return
	}
}
