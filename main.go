package main

import (
	"encoding/json"
	"fmt"

	"github.com/fedepaol/vrfcni/vrf"
	"github.com/vishvananda/netlink"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"

	"github.com/containernetworking/plugins/pkg/ns"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
)

// VrfNetConf represents the firewall configuration.
type VrfNetConf struct {
	types.NetConf

	// Vrf is the name of the vrf to add the interface to.
	Vrf string `json:"vrf"`
	// RoutingTable is the name of the routing table associated to the vrf.
	RoutingTable string `json:"routingtable`
}

func parseConf(data []byte) (*VrfNetConf, *current.Result, error) {
	conf := VrfNetConf{}
	if err := json.Unmarshal(data, &conf); err != nil {
		return nil, nil, fmt.Errorf("failed to load netconf: %v", err)
	}

	if conf.Vrf == "" {
		return nil, nil, fmt.Errorf("configuration is expected to have a valid vrf name")
	}

	// Parse previous result.
	if conf.RawPrevResult == nil {
		// return early if there was no previous result, which is allowed for DEL calls
		return &conf, &current.Result{}, nil
	}

	// Parse previous result.
	var result *current.Result
	var err error
	if err = version.ParsePrevResult(&conf.NetConf); err != nil {
		return nil, nil, fmt.Errorf("could not parse prevResult: %v", err)
	}

	result, err = current.NewResultFromResult(conf.PrevResult)
	if err != nil {
		return nil, nil, fmt.Errorf("could not convert result to current version: %v", err)
	}

	return &conf, result, nil
}

func cmdAdd(args *skel.CmdArgs) error {
	conf, result, err := parseConf(args.StdinData)
	if err != nil {
		return err
	}

	if conf.PrevResult == nil {
		return fmt.Errorf("missing prevResult from earlier plugin")
	}

	err = ns.WithNetNSPath(args.Netns, func(_ ns.NetNS) error {
		v, err := vrf.Find(conf.Vrf)
		if err != nil {
			_, ok := err.(netlink.LinkNotFoundError)
			if !ok {
				return err
			}
			v, err = vrf.Create(conf.Vrf)
			if err != nil {
				return err
			}
		}
		err = vrf.AddInterface(v, args.IfName)
		if err != nil {
			return err
		}
		return nil
	})

	if result == nil {
		result = &current.Result{}
	}

	return types.PrintResult(result, conf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	conf, result, err := parseConf(args.StdinData)
	if err != nil {
		return err
	}
	// TODO

	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.PluginSupports("0.4.0"), bv.BuildString("firewall"))
}

func cmdCheck(args *skel.CmdArgs) error {
	conf, result, err := parseConf(args.StdinData)
	if err != nil {
		return err
	}

	// Ensure we have previous result.
	if conf.PrevResult == nil {
		return fmt.Errorf("missing prevResult from earlier plugin")
	}

	// TODO

	return nil
}
