package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/j-keck/arping"

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
		for key, value := range tuningConf.SysCtl {
			fileName := filepath.Join("/proc/sys", strings.Replace(key, ".", "/", -1))
			fileName = filepath.Clean(fileName)

			// Refuse to modify sysctl parameters that don't belong
			// to the network subsystem.
			if !strings.HasPrefix(fileName, "/proc/sys/net/") {
				return fmt.Errorf("invalid net sysctl key: %q", key)
			}
			content := []byte(value)
			err := ioutil.WriteFile(fileName, content, 0644)
			if err != nil {
				return err
			}
		}

		if tuningConf.Mac != "" {
			if err = changeMacAddr(args.IfName, tuningConf.Mac); err != nil {
				return err
			}

			for _, ipc := range result.IPs {
				if ipc.Version == "4" {
					_ = arping.GratuitousArpOverIfaceByName(ipc.Address.IP, args.IfName)
				}
			}

			updateResultsMacAddr(*tuningConf, args.IfName, tuningConf.Mac)
		}

		if tuningConf.Promisc != false {
			if err = changePromisc(args.IfName, true); err != nil {
				return err
			}
		}

		if tuningConf.Mtu != 0 {
			if err = changeMtu(args.IfName, tuningConf.Mtu); err != nil {
				return err
			}
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
