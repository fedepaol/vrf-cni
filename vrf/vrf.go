package vrf

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

// Find finds a VRF link with the provided name
func Find(name string) (*netlink.Vrf, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}
	vrf, ok := link.(*netlink.Vrf)
	if !ok {
		return nil, fmt.Errorf("Netlink %s is not a VRF", name)
	}
	return vrf, nil
}

// Create creates a new VRF and sets it up
func Create(name string) (*netlink.Vrf, error) {
	tableID, err := findFreeRoutingTableID()
	if err != nil {
		return nil, err
	}
	vrf := &netlink.Vrf{LinkAttrs: netlink.LinkAttrs{
		Name: name,
	},
		Table: tableID}

	err = netlink.LinkAdd(vrf)
	if err != nil {
		return nil, fmt.Errorf("could not add VRF %s: %v", name, err)
	}
	err = netlink.LinkSetUp(vrf)
	if err != nil {
		return nil, fmt.Errorf("could not set link up for VRF %s: %v", name, err)
	}

	return vrf, nil
}

// Retrieve
func getAssignedInterfaces(vrf *netlink.Vrf) ([]netlink.Link, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("getAssignedInterfaces: Failed to find links %v", err)
	}
	res := make([]netlink.Link, 0)
	for _, l := range links {
		if l.Attrs().MasterIndex == vrf.Index {
			res = append(res, l)
		}
	}
	return res, nil
}

// AddInterface adds the given interface to the VRF
func AddInterface(vrf *netlink.Vrf, intf string) error {
	i, err := netlink.LinkByName(intf)
	if err != nil {
		return fmt.Errorf("could not get link by name")
	}
	err = netlink.LinkSetMaster(i, vrf)
	if err != nil {
		return fmt.Errorf("could not set vrf %s as master of %s: %v", vrf.Name, intf, err)
	}
	return nil
}

func findFreeRoutingTableID() (uint32, error) {
	var maxTable uint32
	links, err := netlink.LinkList()
	if err != nil {
		return 0, fmt.Errorf("findFreeRoutingTableID: Failed to find links %v", err)
	}
	for _, l := range links {
		if vrf, ok := l.(*netlink.Vrf); ok {
			if vrf.Table > maxTable {
				maxTable = vrf.Table
			}
		}
	}
	return (maxTable + 1), nil
}
