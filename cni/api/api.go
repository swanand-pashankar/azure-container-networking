package api

import (
	"encoding/json"
	"net"
	"os"

	"github.com/Azure/azure-container-networking/log"
)

type PodNetworkInterfaceInfo struct {
	PodName       string
	PodNamespace  string
	PodEndpointId string
	ContainerID   string
	IPAddresses   []net.IPNet
}

type State struct {
	ContainerInterfaces map[string]PodNetworkInterfaceInfo
}

func (s *State) WriteToStdout() error {
	b, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		log.Errorf("Failed to unmarshall Azure CNI state, err:%v.\n", err)
	}

	// write result to stdout to be captured by caller
	_, err = os.Stdout.Write(b)
	if err != nil {
		log.Printf("Failed to write response to stdout %v", err)
		return err
	}

	return nil
}
