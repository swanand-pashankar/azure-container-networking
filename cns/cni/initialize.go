package cni

import (
	"github.com/Azure/azure-container-networking/cni/api"
	"github.com/Azure/azure-container-networking/cni/client"
	"github.com/Azure/azure-container-networking/cns"
	"k8s.io/utils/exec"
)

// CNICNSInitializer returns an implementation of cns.PodInfoByIPProvider
// that execs out to the CNI and uses the response to build the PodInfo map.
func CNICNSInitializer() (cns.PodInfoByIPProvider, error) {
	cli := client.New(exec.New())
	state, err := cli.GetEndpointState()
	if err != nil {
		return nil, err
	}
	return cns.PodInfoByIPProviderFunc(func() map[string]cns.PodInfo {
		return cniStateToPodInfoByIP(state)
	}), nil
}

// cniStateToPodInfoByIP converts an AzureCNIState dumped from a CNI exec
// into a PodInfo map, using the first endpoint IP as the key in the map.
func cniStateToPodInfoByIP(state *api.State) map[string]cns.PodInfo {
	podInfoByIP := map[string]cns.PodInfo{}
	for _, endpoint := range state.ContainerInterfaces {
		podInfoByIP[endpoint.IPAddresses[0].IP.String()] = cns.NewPodInfo(
			"",
			endpoint.PodEndpointId,
			endpoint.PodName,
			endpoint.PodNamespace,
		)
	}
	return podInfoByIP
}
