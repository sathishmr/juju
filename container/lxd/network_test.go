// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package lxd_test

import (
	"errors"

	"github.com/golang/mock/gomock"
	jc "github.com/juju/testing/checkers"
	lxdapi "github.com/lxc/lxd/shared/api"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/container/lxd"
	lxdtesting "github.com/juju/juju/container/lxd/testing"
	"github.com/juju/juju/network"
)

type networkSuite struct {
	lxdtesting.BaseSuite
}

var _ = gc.Suite(&networkSuite{})

func defaultProfile() *lxdapi.Profile {
	return &lxdapi.Profile{
		Name: "default",
		ProfilePut: lxdapi.ProfilePut{
			Devices: map[string]map[string]string{
				"eth0": {
					"parent":  network.DefaultLXDBridge,
					"type":    "nic",
					"nictype": "bridged",
				},
			},
		},
	}
}

func (s *networkSuite) TestEnsureIPv4NoChange(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "network")

	net := &lxdapi.Network{
		NetworkPut: lxdapi.NetworkPut{
			Config: map[string]string{
				"ipv4.address": "10.5.3.1",
			},
		},
	}
	cSvr.EXPECT().GetNetwork("some-net-name").Return(net, lxdtesting.ETag, nil)

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, jc.ErrorIsNil)

	mod, err := jujuSvr.EnsureIPv4("some-net-name")
	c.Assert(err, jc.ErrorIsNil)
	c.Check(mod, jc.IsFalse)
}

func (s *networkSuite) TestEnsureIPv4Modified(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "network")

	req := lxdapi.NetworkPut{
		Config: map[string]string{
			"ipv4.address": "auto",
			"ipv4.nat":     "true",
		},
	}
	gomock.InOrder(
		cSvr.EXPECT().GetNetwork(network.DefaultLXDBridge).Return(&lxdapi.Network{}, lxdtesting.ETag, nil),
		cSvr.EXPECT().UpdateNetwork(network.DefaultLXDBridge, req, lxdtesting.ETag).Return(nil),
	)

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, jc.ErrorIsNil)

	mod, err := jujuSvr.EnsureIPv4(network.DefaultLXDBridge)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(mod, jc.IsTrue)
}

func (s *networkSuite) TestVerifyNetworkDevicePresentValid(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "network")

	cSvr.EXPECT().GetNetwork(network.DefaultLXDBridge).Return(&lxdapi.Network{}, "", nil)

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, jc.ErrorIsNil)

	err = jujuSvr.VerifyNetworkDevice(defaultProfile(), "")
	c.Assert(err, jc.ErrorIsNil)
}

func (s *networkSuite) TestVerifyNetworkDeviceMultipleNICsOneValid(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerClustered(ctrl, "cluster-1")

	profile := defaultProfile()
	profile.Devices["eno1"] = profile.Devices["eth0"]
	profile.Devices["eno1"]["parent"] = "valid-net"

	net := &lxdapi.Network{
		Name:    network.DefaultLXDBridge,
		Managed: true,
		NetworkPut: lxdapi.NetworkPut{
			Config: map[string]string{
				"ipv6.address": "something-not-nothing",
			},
		},
	}

	// Random map iteration may or may not cause this call to be made.
	cSvr.EXPECT().GetNetwork(network.DefaultLXDBridge).Return(net, "", nil).MaxTimes(1)
	cSvr.EXPECT().GetNetwork("valid-net").Return(&lxdapi.Network{}, "", nil)

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, jc.ErrorIsNil)

	err = jujuSvr.VerifyNetworkDevice(profile, "")
	c.Assert(err, jc.ErrorIsNil)

	c.Check(jujuSvr.LocalBridgeName(), gc.Equals, "valid-net")
}

func (s *networkSuite) TestVerifyNetworkDevicePresentBadNicType(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "network")

	profile := defaultProfile()
	profile.Devices["eth0"]["nictype"] = "not-bridge-or-macvlan"

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, jc.ErrorIsNil)

	err = jujuSvr.VerifyNetworkDevice(profile, "")
	c.Assert(err, gc.ErrorMatches,
		`profile "default": no network device found with nictype "bridged" or "macvlan", `+
			`and without IPv6 configured.\n`+
			`\tthe following devices were checked: \[eth0\]`)
}

func (s *networkSuite) TestVerifyNetworkDeviceIPv6Present(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "network")

	net := &lxdapi.Network{
		Name:    network.DefaultLXDBridge,
		Managed: true,
		NetworkPut: lxdapi.NetworkPut{
			Config: map[string]string{
				"ipv6.address": "something-not-nothing",
			},
		},
	}
	cSvr.EXPECT().GetNetwork(network.DefaultLXDBridge).Return(net, "", nil)

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, jc.ErrorIsNil)

	err = jujuSvr.VerifyNetworkDevice(defaultProfile(), "")
	c.Assert(err, gc.ErrorMatches,
		`profile "default": no network device found with nictype "bridged" or "macvlan", `+
			`and without IPv6 configured.\n`+
			`\tthe following devices were checked: \[eth0\]`)
}

func (s *networkSuite) TestVerifyNetworkDeviceNotPresentCreated(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "network")

	netConf := map[string]string{
		"ipv4.address": "auto",
		"ipv4.nat":     "true",
		"ipv6.address": "none",
		"ipv6.nat":     "false",
	}
	netCreateReq := lxdapi.NetworksPost{
		Name:       network.DefaultLXDBridge,
		Type:       "bridge",
		Managed:    true,
		NetworkPut: lxdapi.NetworkPut{Config: netConf},
	}
	newNet := &lxdapi.Network{
		Name:       network.DefaultLXDBridge,
		Type:       "bridge",
		Managed:    true,
		NetworkPut: lxdapi.NetworkPut{Config: netConf},
	}
	gomock.InOrder(
		cSvr.EXPECT().GetNetwork(network.DefaultLXDBridge).Return(nil, "", errors.New("not found")),
		cSvr.EXPECT().CreateNetwork(netCreateReq).Return(nil),
		cSvr.EXPECT().GetNetwork(network.DefaultLXDBridge).Return(newNet, "", nil),
		cSvr.EXPECT().UpdateProfile("default", defaultProfile().Writable(), lxdtesting.ETag).Return(nil),
	)

	profile := defaultProfile()
	delete(profile.Devices, "eth0")

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, jc.ErrorIsNil)

	err = jujuSvr.VerifyNetworkDevice(profile, lxdtesting.ETag)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *networkSuite) TestVerifyNetworkDeviceNotPresentCreatedWithUnusedName(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerWithExtensions(ctrl, "network")

	defaultBridge := &lxdapi.Network{
		Name:    network.DefaultLXDBridge,
		Type:    "bridge",
		Managed: true,
		NetworkPut: lxdapi.NetworkPut{
			Config: map[string]string{
				"ipv4.address": "auto",
				"ipv4.nat":     "true",
				"ipv6.address": "none",
				"ipv6.nat":     "false",
			},
		},
	}
	devReq := lxdapi.ProfilePut{
		Devices: map[string]map[string]string{
			"eth0": {},
			"eth1": {},
			// eth2 will be generated as the first unused device name.
			"eth2": {
				"parent":  network.DefaultLXDBridge,
				"type":    "nic",
				"nictype": "bridged",
			},
		},
	}
	gomock.InOrder(
		cSvr.EXPECT().GetNetwork(network.DefaultLXDBridge).Return(defaultBridge, "", nil),
		cSvr.EXPECT().UpdateProfile("default", devReq, lxdtesting.ETag).Return(nil),
	)

	profile := defaultProfile()
	profile.Devices["eth0"] = map[string]string{}
	profile.Devices["eth1"] = map[string]string{}

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, jc.ErrorIsNil)

	err = jujuSvr.VerifyNetworkDevice(profile, lxdtesting.ETag)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *networkSuite) TestVerifyNetworkDeviceNotPresentErrorForCluster(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServerClustered(ctrl, "cluster-1")

	profile := defaultProfile()
	delete(profile.Devices, "eth0")

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, jc.ErrorIsNil)

	err = jujuSvr.VerifyNetworkDevice(profile, lxdtesting.ETag)
	c.Assert(err, gc.ErrorMatches, `profile "default" does not have any devices configured with type "nic"`)
}

func (s *networkSuite) TestCheckLXDBridgeConfiguration(c *gc.C) {
	bridgeName, err := lxd.CheckBridgeConfigFile(validBridgeConfig)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(bridgeName, gc.Equals, "lxdbr0")

	noBridge := func(string) ([]byte, error) {
		return []byte(`
USE_LXD_BRIDGE="false"
`), nil
	}
	_, err = lxd.CheckBridgeConfigFile(noBridge)
	c.Assert(err.Error(), gc.Equals, `/etc/default/lxd-bridge has USE_LXD_BRIDGE set to false
It looks like your LXD bridge has not yet been configured. Configure it via:

	sudo dpkg-reconfigure -p medium lxd

and run the command again.`)

	noSubnets := func(string) ([]byte, error) {
		return []byte(`
USE_LXD_BRIDGE="true"
LXD_BRIDGE="br0"
`), nil
	}
	_, err = lxd.CheckBridgeConfigFile(noSubnets)
	c.Assert(err.Error(), gc.Equals, `br0 has no ipv4 or ipv6 subnet enabled
It looks like your LXD bridge has not yet been configured. Configure it via:

	sudo dpkg-reconfigure -p medium lxd

and run the command again.`)

	ipv6 := func(string) ([]byte, error) {
		return []byte(`
USE_LXD_BRIDGE="true"
LXD_BRIDGE="lxdbr0"
LXD_IPV6_ADDR="2001:470:b368:4242::1"
`), nil
	}

	_, err = lxd.CheckBridgeConfigFile(ipv6)
	c.Assert(err.Error(), gc.Equals, lxd.BridgeConfigFile+` has IPv6 enabled.
Juju doesn't currently support IPv6.
Disable IPv6 via:

	sudo dpkg-reconfigure -p medium lxd

and run the command again.`)
}

func (s *networkSuite) TestVerifyNICsWithConfigFileNICFound(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServer(ctrl)

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, jc.ErrorIsNil)

	err = lxd.VerifyNICsWithConfigFile(jujuSvr, defaultProfile().Devices, validBridgeConfig)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(jujuSvr.LocalBridgeName(), gc.Equals, "lxdbr0")
}

func (s *networkSuite) TestVerifyNICsWithConfigFileNICNotFound(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	cSvr := s.NewMockServer(ctrl)

	jujuSvr, err := lxd.NewServer(cSvr)
	c.Assert(err, jc.ErrorIsNil)

	nics := defaultProfile().Devices
	nics["eth0"]["parent"] = "br0"

	err = lxd.VerifyNICsWithConfigFile(jujuSvr, nics, validBridgeConfig)
	c.Assert(err, gc.ErrorMatches,
		`no network device found with nictype "bridged" or "macvlan" that uses the configured bridge in `+
			lxd.BridgeConfigFile+"\n\tthe following devices were checked: "+`\[eth0\]`)
}

func validBridgeConfig(_ string) ([]byte, error) {
	return []byte(`
# Whether to setup a new bridge or use an existing one
USE_LXD_BRIDGE="true"

# Bridge name
# This is still used even if USE_LXD_BRIDGE is set to false
# set to an empty value to fully disable
LXD_BRIDGE="lxdbr0"

# Path to an extra dnsmasq configuration file
LXD_CONFILE=""

# DNS domain for the bridge
LXD_DOMAIN="lxd"

# IPv4
## IPv4 address (e.g. 10.0.4.1)
LXD_IPV4_ADDR="10.0.4.1"

## IPv4 netmask (e.g. 255.255.255.0)
LXD_IPV4_NETMASK="255.255.255.0"

## IPv4 network (e.g. 10.0.4.0/24)
LXD_IPV4_NETWORK="10.0.4.1/24"

## IPv4 DHCP range (e.g. 10.0.4.2,10.0.4.254)
LXD_IPV4_DHCP_RANGE="10.0.4.2,10.0.4.254"

## IPv4 DHCP number of hosts (e.g. 250)
LXD_IPV4_DHCP_MAX="253"

## NAT IPv4 traffic
LXD_IPV4_NAT="true"

# IPv6
## IPv6 address (e.g. 2001:470:b368:4242::1)
LXD_IPV6_ADDR=""

## IPv6 CIDR mask (e.g. 64)
LXD_IPV6_MASK=""
LXD_IPV6_NETWORK=""
`), nil
}
