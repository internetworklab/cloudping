package ipinfo

import (
	"context"
	"math/rand"
)

type RandomIPInfoAdapter struct {
	name string
}

func NewRandomIPInfoAdapter(name string) GeneralIPInfoAdapter {
	return &RandomIPInfoAdapter{name: name}
}

func (ia *RandomIPInfoAdapter) GetIPInfo(ctx context.Context, ip string) (*BasicIPInfo, error) {
	randASNList := []string{
		"AS65001",
		"AS65002",
		"AS65003",
		"AS65004",
		"AS65005",
		"AS65006",
	}

	randISPList := []string{
		"CT",
		"CM",
		"CU",
	}

	randLocationList := []string{
		"CN",
		"US",
		"JP",
		"KR",
		"TW",
		"HK",
		"MO",
	}

	randASN := randASNList[rand.Intn(len(randASNList))]
	randISP := randISPList[rand.Intn(len(randISPList))]
	randLocation := randLocationList[rand.Intn(len(randLocationList))]

	return &BasicIPInfo{
		ASN:      randASN,
		ISP:      randISP,
		Location: randLocation,
		Exact:    RandomLocation(),
	}, nil
}

func (ia *RandomIPInfoAdapter) GetName() string {
	if name := ia.name; name != "" {
		return name
	}
	return "random"
}
