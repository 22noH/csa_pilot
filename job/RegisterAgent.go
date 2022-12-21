package job

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

func GetMacAddress() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}

	var currentIP, currentNetworkHardwareName string

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				currentIP = ipnet.IP.String()
			}
		}
	}
	fmt.Println("ip : ", currentIP)
	interfaces, _ := net.Interfaces()
	for _, interf := range interfaces {
		if addrs, err := interf.Addrs(); err == nil {
			for _, addr := range addrs {
				if strings.Contains(addr.String(), currentIP) {
					currentNetworkHardwareName = interf.Name
				}
			}
		}
	}

	netInterface, err := net.InterfaceByName(currentNetworkHardwareName)
	if err != nil {
		panic(err)
	}
	macaddr := netInterface.HardwareAddr.String()
	return macaddr

}

type AgentInfo struct {
	NodeName     string `json:"NodeName"`
	PodName      string `json:"PodName"`
	PodNameSpace string `json:"PodNameSpace"`
	PodUID       string `json:"PodUID"`
	NodeIP       string `json:"NodeIP"`
	PodIP        string `json:"PodIP"`
	MacAdress    string `json:"MacAdress"`
}

func RegisterAgent() string {
	var agentinfo AgentInfo
	agentinfo.NodeName = os.Getenv("CSA_NODE_NAME")
	agentinfo.PodName = os.Getenv("CSA_POD_NAME")
	agentinfo.PodNameSpace = os.Getenv("CSA_POD_NAMESPACE")
	agentinfo.PodUID = os.Getenv("CSA_POD_UID")
	agentinfo.PodIP = os.Getenv("CSA_POD_IP")
	agentinfo.NodeIP = os.Getenv("CSA_NODE_IP")
	agentinfo.MacAdress = GetMacAddress()

	jsonMerged, err := json.MarshalIndent(agentinfo, "", "  ")
	if err != nil {
		panic(err)
	}

	return string(jsonMerged)
}
