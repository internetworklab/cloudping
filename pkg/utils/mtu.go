package utils

import (
	"net"
)

func GetMaximumMTU() int {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	maximumMTU := -1
	for _, iface := range ifaces {
		if iface.MTU > maximumMTU {
			maximumMTU = iface.MTU
		}
	}
	if maximumMTU == -1 {
		panic("can't determine maximum MTU")
	}
	return maximumMTU
}

const standardMTU int = 1500

func GetMinimumMTU() int {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	mtuVals := make([]int, 0)
	for _, iface := range ifaces {
		mtuVals = append(mtuVals, iface.MTU)
	}

	if len(mtuVals) == 0 {
		return standardMTU
	}

	minMTU := mtuVals[0]
	for _, mtu := range mtuVals {
		if mtu < minMTU {
			minMTU = mtu
		}
	}

	return minMTU
}
