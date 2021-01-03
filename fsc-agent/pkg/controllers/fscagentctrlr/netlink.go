package fscagentctrlr

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/fsc-demo-wim/netlinkd/netlinktypes"
	"github.com/vishvananda/netlink"
)

// CheckNetLinkDaemon function validates connectivity to the netlink daemon
func CheckNetLinkDaemon() error {
	conn, err := net.Dial("unix", "/tmp/netlink.sock")
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte("give me netlink hosts status"))
	if err != nil {
		log.Fatal("write error:", err)
	}

	d := json.NewDecoder(conn)
	var nll []netlinktypes.Link
	err = d.Decode(&nll)

	if err != nil {
		return err
	}
	return nil
}

// get the netlink information via the linux socket
func getNetLinkInfoFromHost() (*[]netlinktypes.Link, error) {
	conn, err := net.Dial("unix", "/tmp/netlink.sock")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_, err = conn.Write([]byte("give me netlink hosts status"))
	if err != nil {
		log.Fatal("write error:", err)
	}

	d := json.NewDecoder(conn)
	var nll []netlinktypes.Link
	err = d.Decode(&nll)

	if err != nil {
		return nil, err
	}
	return &nll, nil
}

// UpdateNetLinkCache function
func (c *workerController) UpdateNetLinkCache() error {
	nll, err := getNetLinkInfoFromHost()
	if err != nil {
		return err
	}
	// loop over the multus information
	for netwAttName, na := range *c.multusConfig {
		for _, l := range *nll {
			log.Infof("NetLink: %s, %d %d", l.Name, l.ParentIndex, l.MasterIndex)
			switch na.Kind {
			case "sriov":
				// We only care about interfaces that are configured through the multus
				// configuration as network attachement definitions;
				// Check if the multus interface name and netlink interface name match
				if na.ItfceName == l.Name {
					if cl, ok := (*c.linkCache)[l.Name]; ok {
						// link exists in cache
						// check if vf changed
						for _, vf := range l.Vfs {
							if vf.Vlan != 0 {
								if la, ok := cl[vf.Vlan]; ok {
									var ns string
									// for SRIOV the namespace is tied in multus config
									// we lookup multiple vlans from the ifName which gives wrong results
									// we need to lookup back in the multus config db to find the namespace
									for _, v := range *c.multusConfig {
										if v.ItfceName == l.Name && v.Vlan == vf.Vlan {
											ns = v.Ns
										}
									}
									c.updateSocketLinkVlanInCache(l, la, ns)
								}
							}
						}
					} else {
						// link does not exists in cache
						(*c.linkCache)[l.Name] = make(map[int]*LinkAttr)
						for _, vf := range l.Vfs {
							if vf.Vlan != 0 {
								c.addSocketLinkVlan2Cache(l, vf.Vlan, netwAttName)
							}
						}
					}
				}
			case "ipvlan", "macvlan":
				// We only care about interfaces that are configured through the multus
				// configuration as network attachement definitions;
				// check if the multus interface name and netlink interface name match.
				// For ipvlan we use <interfaceName>.<vlan> notation
				if na.ItfceName+"."+strconv.Itoa(na.Vlan) == l.Name {

					if cl, ok := (*c.linkCache)[l.Name]; ok {
						// link exists in cache
						if la, ok := cl[na.Vlan]; ok {
							c.updateSocketLinkVlanInCache(l, la, netwAttName)
						}
					} else {
						// link does not exists in cache
						(*c.linkCache)[l.Name] = make(map[int]*LinkAttr)
						c.addSocketLinkVlan2Cache(l, na.Vlan, netwAttName)
					}
				}
			}
		}
		// update slave interfaces for bonding
		// loop over the link cache
		for ifName, v := range *c.linkCache {
			// loop over the vlans in the link cache
			for _, la := range v {
				// if the ChangeFlag did not change it means we should delete this link
				// and hence we dont process it here since it can give wrong information
				if la.ChangeFlag != "" {
					// a bonded interface will have a parent index != 0
					if la.ParentIndex != 0 {
						// we search for the corresponding slave interface by matching
						// the master index to the parent index
						// e.g. bonded interface has parent index 10
						// slave interfaces have master index 10
						origSlaveLinks := la.SlaveLinks
						la.SlaveLinks = make([]string, 0)
						for _, l := range *nll {
							if l.MasterIndex == la.ParentIndex {
								la.SlaveLinks = append(la.SlaveLinks, l.Name)
							}
						}
						// check if there is a change in the slave link
						if !reflect.DeepEqual(origSlaveLinks, la.SlaveLinks) {
							la.ChangeFlag = "stateChange"
						}
					} else {
						// copy the ifname to the slave Link to make further processign easier
						la.SlaveLinks = make([]string, 0)
						la.SlaveLinks = append(la.SlaveLinks, ifName)
					}
				}
			}
		}
	}
	return nil
}

func (c *workerController) addSocketLinkVlan2Cache(l netlinktypes.Link, vlan int, netwAttName string) {
	(*c.linkCache)[l.Name][vlan] = &LinkAttr{
		ChangeFlag:   "newlyAdded",
		HardwareAddr: l.HardwareAddr,
		MTU:          l.MTU,
		OperState:    l.OperState,
		ParentIndex:  l.ParentIndex,
		NameSpace:    getNameSpaceFromString(netwAttName),
	}
}

func (c *workerController) addLinkVlan2Cache(l netlink.Link, vlan int, netwAttName string) {
	(*c.linkCache)[l.Attrs().Name][vlan] = &LinkAttr{
		ChangeFlag:   "newlyAdded",
		HardwareAddr: l.Attrs().HardwareAddr.String(),
		MTU:          l.Attrs().MTU,
		OperState:    l.Attrs().OperState.String(),
		ParentIndex:  l.Attrs().ParentIndex,
		NameSpace:    getNameSpaceFromString(netwAttName),
	}
}

func getNameSpaceFromString(netwAttName string) string {
	return strings.Split(netwAttName, "/")[0]
}

func (c *workerController) updateSocketLinkVlanInCache(l netlinktypes.Link, la *LinkAttr, netwAttName string) {
	la.ChangeFlag = "noChange"
	if la.HardwareAddr != l.HardwareAddr {
		la.ChangeFlag = "stateChange"
	}
	if la.MTU != l.MTU {
		log.Debugf("MTU CHANGE: %v, %v ", la.MTU, l.MTU)
		la.ChangeFlag = "configChange"
	}
	if la.OperState != l.OperState {
		log.Debugf("OPERSTATE CHANGE: %v, %v ", la.OperState, l.OperState)
		la.ChangeFlag = "configChange"
	}
	if la.ParentIndex != l.ParentIndex {
		log.Debugf("PARENTINDEX CHANGE: %v, %v ", la.ParentIndex, l.ParentIndex)
		la.ChangeFlag = "configChange"
	}
	/*
		if la.NameSpace != getNameSpaceFromString(netwAttName) {
			log.Debugf("NAMESPACE CHANGE: %v, %v ", la.NameSpace,  getNameSpaceFromString(netwAttName))
			la.ChangeFlag = "configChange"
		}
	*/
}

func (c *workerController) updateLinkVlanInCache(l netlink.Link, la *LinkAttr, netwAttName string) {
	la.ChangeFlag = "noChange"
	if la.HardwareAddr != l.Attrs().HardwareAddr.String() {
		la.ChangeFlag = "stateChange"
	}
	if la.MTU != l.Attrs().MTU {
		log.Debugf("MTU CHANGE: %v, %v ", la.MTU, l.Attrs().MTU)
		la.ChangeFlag = "configChange"
	}
	if la.OperState != l.Attrs().OperState.String() {
		log.Debugf("OPERSTATE CHANGE: %v, %v ", la.OperState, l.Attrs().OperState.String())
		la.ChangeFlag = "configChange"
	}
	if la.ParentIndex != l.Attrs().ParentIndex {
		log.Debugf("PARENTINDEX CHANGE: %v, %v ", la.ParentIndex, l.Attrs().ParentIndex)
		la.ChangeFlag = "configChange"
	}
	/*
		if la.NameSpace != getNameSpaceFromString(netwAttName) {
			log.Debugf("NAMESPACE CHANGE: %v, %v ", la.NameSpace,  getNameSpaceFromString(netwAttName))
			la.ChangeFlag = "configChange"
		}
	*/
}

// ValidateNetLinkCacheChanges function
func (c *workerController) ValidateNetLinkCacheChanges() error {
	fmt.Println("ValidateLinkCacheChanges Start....")
	for itfceName, v := range *c.linkCache {
		log.Debugf("Link Cache: %s", itfceName)
		for vlan, la := range v {
			// validate change flag
			// when empty: "" -> the link no longer exists
			// for others we indicate the change in LldpChangeFlag flag to indicate
			// changes to the next processing level
			switch la.ChangeFlag {
			case "":
				la.ChangeFlag = "toBeDeleted"
				break
			case "noChange":
				la.LldpChangeFlag = false
			default:
				la.LldpChangeFlag = true
			}

			log.Debugf("  VLAN: %d Attributes: %v", vlan, *la)
			// if the vlan no longer exists delete it from the cache
			if la.ChangeFlag == "toBeDeleted" {
				delete((*c.linkCache)[itfceName], vlan)
			}
			la.ChangeFlag = ""
		}
		// if there are no vlans only longer on the links delete the link
		if len((*c.linkCache)[itfceName]) == 0 {
			delete(*c.linkCache, itfceName)
		}
	}
	fmt.Println("ValidateLinkCacheChanges Stop ...")
	return nil
}
