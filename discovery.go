package ZookeeperServiceDiscovery

import (
	"encoding/json"
	"google.golang.org/grpc/grpclog"
	"io"
	"net/http"
	"os"
)

type ecsZkDiscovery struct {
	zk     *zkClient
	schema string
	path   string
}

func NewEcsZkServiceDiscovery(zkServers []string, zkTimeout int, registrationPath string, args ...string) (*ecsZkDiscovery, error) {
	var schema string
	if len(args) > 0 {
		schema = args[0]
	} else {
		schema = "zk"
	}

	zkClient, err := NewZkClient(zkServers, zkTimeout)

	if err != nil {
		return nil, err
	}

	sd := &ecsZkDiscovery{
		zk:     zkClient,
		schema: schema,
		path:   registrationPath,
	}

	return sd, nil
}

func (sd *ecsZkDiscovery) RegisterService() error {
	metadataEndpoint, isEcs := os.LookupEnv("ECS_CONTAINER_METADATA_URI_V4")

	if !isEcs {
		grpclog.Infof("Not running inside ECS")
		return nil
	}

	var ip string

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

	return sd.zk.RegisterNode(sd.path, ip)
}

func (sd *ecsZkDiscovery) Unregister() {
	sd.zk.Close()
}
