package ZookeeperServiceDiscovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
	"io"
	"net"
	"net/http"
	"os"
)

type EcsZkDiscovery struct {
	zk     *zkClient
	schema string
	path   string
}

func NewEcsZkServiceDiscovery(zkServers []string, zkTimeout int, registrationPath string, args ...string) (*EcsZkDiscovery, error) {
	var schema string
	if len(args) > 0 {
		schema = args[0]
	} else {
		schema = "zk"
	}

	zkClient, err := newZkClient(zkServers, zkTimeout)

	if err != nil {
		return nil, err
	}

	sd := &EcsZkDiscovery{
		zk:     zkClient,
		schema: schema,
		path:   registrationPath,
	}

	return sd, nil
}

func (sd *EcsZkDiscovery) RegisterService(port string, args ...string) error {
	metadataEndpoint, isEcs := os.LookupEnv("ECS_CONTAINER_METADATA_URI_V4")

	var ip string

	if len(args) > 0 {
		ip = args[0]
	} else if isEcs {
		grpclog.Infof("Running inside ECS, getting IP from ECS metadata endpoint")

		resp, err := http.Get(metadataEndpoint)
		if err != nil {
			grpclog.Errorf("Failed to get ECS metadata: %v", err)
			return err
		}

		b, err := io.ReadAll(resp.Body)

		if err != nil {
			grpclog.Errorf("Failed to get ECS metadata body: %v", err)
			return err
		}

		var jsonData interface{}
		json.Unmarshal(b, &jsonData)
		m := jsonData.(map[string]interface{})

		network := m["Networks"].([]interface{})[0].(map[string]interface{})
		networkMode := network["NetworkMode"].(string)

		if networkMode == "awsvpc" {
			ip = network["IPv4Addresses"].([]interface{})[0].(string)
		} else {
			// get EC2 instance IP
			ec2MetadataEndpoint, isEnvVar := os.LookupEnv("EC2_METADATA_IP")

			if !isEnvVar {
				ec2MetadataEndpoint = "http://169.254.169.254"
			}

			resp, err := http.Get(ec2MetadataEndpoint + "/latest/meta-data/local-ipv4")
			if err != nil {
				grpclog.Errorf("Failed to get EC2 metadata: %v", err)
				return err
			}

			b, err := io.ReadAll(resp.Body)

			if err != nil {
				grpclog.Errorf("Failed to get ECS metadata body: %v", err)
				return err
			}

			ip = string(b)
		}
	} else {
		grpclog.Infof("Getting first public IP from interfaces")
		addrs, err := net.InterfaceAddrs()

		if err != nil {
			grpclog.Errorf("Error while getting IP: %v", err)
			return err
		}

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if ok && !ipnet.IP.IsLoopback() {
				ip = ipnet.IP.String()
				break
			}
		}
	}

	if ip == "" {
		return errors.New("Could not get public IP address")
	}

	return sd.zk.RegisterNode(sd.path, fmt.Sprintf("%s:%s", ip, port))
}

func (sd *EcsZkDiscovery) Unregister() {
	sd.zk.Close()
}

func (sd *EcsZkDiscovery) RegisterResolver() {
	resolver.Register(&zkResolver{
		scheme: sd.schema,
		zk:     sd.zk,
		path:   sd.path,
	})
}
