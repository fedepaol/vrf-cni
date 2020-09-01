package main

import (
	"encoding/json"
	"fmt"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/testutils"

	"github.com/vishvananda/netlink"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func buildOneConfig(name, cniVersion string, orig *VrfNetConf, prevResult types.Result) (*VrfNetConf, []byte, error) {
	var err error

	inject := map[string]interface{}{
		"name":       name,
		"cniVersion": cniVersion,
	}
	// Add previous plugin result
	if prevResult != nil {
		inject["prevResult"] = prevResult
	}

	// Ensure every config uses the same name and version
	config := make(map[string]interface{})

	confBytes, err := json.Marshal(orig)
	if err != nil {
		return nil, nil, err
	}

	err = json.Unmarshal(confBytes, &config)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshal existing network bytes: %s", err)
	}

	for key, value := range inject {
		config[key] = value
	}

	newBytes, err := json.Marshal(config)
	if err != nil {
		return nil, nil, err
	}

	conf := &VrfNetConf{}
	if err := json.Unmarshal(newBytes, &conf); err != nil {
		return nil, nil, fmt.Errorf("error parsing configuration: %s", err)
	}

	return conf, newBytes, nil

}

var _ = Describe("vrf plugin", func() {
	var originalNS ns.NetNS
	var targetNS ns.NetNS
	const (
		IF0Name  = "dummy0"
		IF1Name  = "dummy1"
		VRF0Name = "vrf0"
		VRF1Name = "vrf1"
	)

	BeforeEach(func() {
		var err error
		originalNS, err = testutils.NewNS()
		Expect(err).NotTo(HaveOccurred())

		targetNS, err = testutils.NewNS()
		Expect(err).NotTo(HaveOccurred())

		err = targetNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			err = netlink.LinkAdd(&netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: IF0Name,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			_, err = netlink.LinkByName(IF0Name)
			Expect(err).NotTo(HaveOccurred())

			err = netlink.LinkAdd(&netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: IF1Name,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			_, err = netlink.LinkByName(IF0Name)
			Expect(err).NotTo(HaveOccurred())
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(originalNS.Close()).To(Succeed())
		Expect(targetNS.Close()).To(Succeed())
	})

	It("passes prevResult through unchanged", func() {
		conf := confFor("test", IF0Name, VRF0Name, "10.0.0.2")

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       targetNS.Path(),
			IfName:      IF0Name,
			StdinData:   conf,
		}

		err := originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			r, _, err := testutils.CmdAddWithArgs(args, func() error {
				return cmdAdd(args)
			})
			Expect(err).NotTo(HaveOccurred())

			result, err := current.GetResult(r)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(result.Interfaces)).To(Equal(1))
			Expect(result.Interfaces[0].Name).To(Equal(IF0Name))
			Expect(len(result.IPs)).To(Equal(1))
			Expect(result.IPs[0].Address.String()).To(Equal("10.0.0.2/24"))
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("configures a VRF and adds the interface to it", func() {
		conf := confFor("test", IF0Name, VRF0Name, "10.0.0.2")

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       targetNS.Path(),
			IfName:      IF0Name,
			StdinData:   conf,
		}

		err := originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()
			_, _, err := testutils.CmdAddWithArgs(args, func() error {
				return cmdAdd(args)
			})
			Expect(err).NotTo(HaveOccurred())
			return nil
		})

		err = targetNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()
			vrf, err := netlink.LinkByName(VRF0Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(vrf).To(BeAssignableToTypeOf(&netlink.Vrf{}))

			link, err := netlink.LinkByName(IF0Name)
			Expect(err).NotTo(HaveOccurred())
			masterIndx := link.Attrs().MasterIndex
			master, err := netlink.LinkByIndex(masterIndx)
			Expect(err).NotTo(HaveOccurred())
			Expect(master.Attrs().Name).To(Equal(VRF0Name))
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("configures a VRF and adds two interfaces to it", func() {
		conf := confFor("test", IF0Name, VRF0Name, "10.0.0.2")
		conf1 := confFor("test1", IF1Name, VRF0Name, "10.0.0.2")

		err := originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()
			args := &skel.CmdArgs{
				ContainerID: "dummy",
				Netns:       targetNS.Path(),
				IfName:      IF0Name,
				StdinData:   conf,
			}
			_, _, err := testutils.CmdAddWithArgs(args, func() error {
				return cmdAdd(args)
			})
			Expect(err).NotTo(HaveOccurred())
			return nil
		})

		err = targetNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()
			vrf, err := netlink.LinkByName(VRF0Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(vrf).To(BeAssignableToTypeOf(&netlink.Vrf{}))

			link, err := netlink.LinkByName(IF0Name)
			Expect(err).NotTo(HaveOccurred())
			masterIndx := link.Attrs().MasterIndex
			master, err := netlink.LinkByIndex(masterIndx)
			Expect(err).NotTo(HaveOccurred())
			Expect(master.Attrs().Name).To(Equal(VRF0Name))
			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		err = originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()
			args := &skel.CmdArgs{
				ContainerID: "dummy",
				Netns:       targetNS.Path(),
				IfName:      IF1Name,
				StdinData:   conf1,
			}
			_, _, err := testutils.CmdAddWithArgs(args, func() error {
				return cmdAdd(args)
			})
			Expect(err).NotTo(HaveOccurred())
			return nil
		})

		err = targetNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()
			vrf, err := netlink.LinkByName(VRF0Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(vrf).To(BeAssignableToTypeOf(&netlink.Vrf{}))

			link, err := netlink.LinkByName(IF1Name)
			Expect(err).NotTo(HaveOccurred())
			masterIndx := link.Attrs().MasterIndex
			master, err := netlink.LinkByIndex(masterIndx)
			Expect(err).NotTo(HaveOccurred())
			Expect(master.Attrs().Name).To(Equal(VRF0Name))
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})

	/*	It("configures and deconfigures promiscuous mode with CNI 0.4.0 ADD/DEL", func() {
				conf := []byte(`{
			"name": "test",
			"type": "iplink",
			"cniVersion": "0.4.0",
			"promisc": true,
			"prevResult": {
				"interfaces": [
					{"name": "dummy0", "sandbox":"netns"}
				],
				"ips": [
					{
						"version": "4",
						"address": "10.0.0.2/24",
						"gateway": "10.0.0.1",
						"interface": 0
					}
				]
			}
		}`)

				args := &skel.CmdArgs{
					ContainerID: "dummy",
					Netns:       originalNS.Path(),
					IfName:      IFNAME,
					StdinData:   conf,
				}

				err := originalNS.Do(func(ns.NetNS) error {
					defer GinkgoRecover()

					r, _, err := testutils.CmdAddWithArgs(args, func() error {
						return cmdAdd(args)
					})
					Expect(err).NotTo(HaveOccurred())

					result, err := current.GetResult(r)
					Expect(err).NotTo(HaveOccurred())

					Expect(len(result.Interfaces)).To(Equal(1))
					Expect(result.Interfaces[0].Name).To(Equal(IFNAME))
					Expect(len(result.IPs)).To(Equal(1))
					Expect(result.IPs[0].Address.String()).To(Equal("10.0.0.2/24"))

					link, err := netlink.LinkByName(IFNAME)
					Expect(err).NotTo(HaveOccurred())
					Expect(link.Attrs().Promisc).To(Equal(1))

					n := &VrfNetConf{}
					err = json.Unmarshal([]byte(conf), &n)
					Expect(err).NotTo(HaveOccurred())

					cniVersion := "0.4.0"
					_, confString, err := buildOneConfig("testConfig", cniVersion, n, r)
					Expect(err).NotTo(HaveOccurred())

					args.StdinData = confString

					err = testutils.CmdCheckWithArgs(args, func() error {
						return cmdCheck(args)
					})
					Expect(err).NotTo(HaveOccurred())

					err = testutils.CmdDel(originalNS.Path(),
						args.ContainerID, "", func() error { return cmdDel(args) })
					Expect(err).NotTo(HaveOccurred())

					return nil
				})
				Expect(err).NotTo(HaveOccurred())
			})*/

})

func confFor(name, intf, vrf, ip string) []byte {
	conf := fmt.Sprintf(`{
		"name": "%s",
		"type": "vrf",
		"cniVersion": "0.3.1",
		"vrfName": "%s",
		"prevResult": {
			"interfaces": [
				{"name": "%s", "sandbox":"netns"}
			],
			"ips": [
				{
					"version": "4",
					"address": "%s/24",
					"gateway": "10.0.0.1",
					"interface": 0
				}
			]
		}
	}`, name, vrf, intf, ip)
	return []byte(conf)
}
