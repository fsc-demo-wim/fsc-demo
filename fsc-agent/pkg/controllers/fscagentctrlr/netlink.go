package fscagentctrlr

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"
)

// UpdateLinkCache function
func (c *workerController) UpdateLinkCache() error {
	nll, err := netlink.LinkList()
	if err != nil {
		return err
	}
	for netwAttName, na := range *c.multusConfig {
		for _, l := range nll {
			//fmt.Printf("NetLink: %s, %d %d\n", l.Attrs().Name, l.Attrs().ParentIndex, l.Attrs().MasterIndex)
			switch na.Kind {
			case "sriov":
				// We only care about interfaces that are configured through the multus
				// configuration as network attachement definitions;
				// Check if the multus interface name and netlink interface name match
				if na.ItfceName == l.Attrs().Name {
					if cl, ok := (*c.linkCache)[l.Attrs().Name]; ok {
						// link exists in cache
						// check if vf changed
						for _, vf := range l.Attrs().Vfs {
							if vf.Vlan != 0 {
								if la, ok := cl[vf.Vlan]; ok {
									var ns string
									// for SRIOV the namespace is tied in multus config
									// we lookup multiple vlans from the ifName which gives wrong results
									// we need to lookup back in the multus config db to find the namespace
									for _, v := range *c.multusConfig {
										if v.ItfceName ==  l.Attrs().Name && v.Vlan == vf.Vlan {
											ns = v.Ns
										}
									}
									c.updateLinkVlanInCache(l, la, ns)
								}
							}
						}
					} else {
						// link does not exists in cache
						(*c.linkCache)[l.Attrs().Name] = make(map[int]*LinkAttr)
						for _, vf := range l.Attrs().Vfs {
							if vf.Vlan != 0 {
								c.addLinkVlan2Cache(l, vf.Vlan, netwAttName)
							}
						}
					}
				}
			case "ipvlan", "macvlan":
				// We only care about interfaces that are configured through the multus
				// configuration as network attachement definitions;
				// check if the multus interface name and netlink interface name match.
				// For ipvlan we use <interfaceName>.<vlan> notation
				if na.ItfceName+"."+strconv.Itoa(na.Vlan) == l.Attrs().Name {

					if cl, ok := (*c.linkCache)[l.Attrs().Name]; ok {
						// link exists in cache
						if la, ok := cl[na.Vlan]; ok {
							c.updateLinkVlanInCache(l, la, netwAttName)
						}
					} else {
						// link does not exists in cache
						(*c.linkCache)[l.Attrs().Name] = make(map[int]*LinkAttr)
						c.addLinkVlan2Cache(l, na.Vlan, netwAttName)
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
						for _, l := range nll {
							if l.Attrs().MasterIndex == la.ParentIndex {
								la.SlaveLinks = append(la.SlaveLinks, l.Attrs().Name)
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

func (c *workerController) updateLinkVlanInCache(l netlink.Link, la *LinkAttr, netwAttName string) {
	la.ChangeFlag = "noChange"
	if la.HardwareAddr != l.Attrs().HardwareAddr.String() {
		la.ChangeFlag = "stateChange"
	}
	if la.MTU != l.Attrs().MTU {
		fmt.Printf("MTU CHANGE: %v, %v \n", la.MTU, l.Attrs().MTU)
		la.ChangeFlag = "configChange"
	}
	if la.OperState != l.Attrs().OperState.String() {
		fmt.Printf("OPERSTATE CHANGE: %v, %v \n", la.OperState, l.Attrs().OperState.String())
		la.ChangeFlag = "configChange"
	}
	if la.ParentIndex != l.Attrs().ParentIndex {
		fmt.Printf("PARENTINDEX CHANGE: %v, %v \n", la.ParentIndex, l.Attrs().ParentIndex)
		la.ChangeFlag = "configChange"
	}
	/*
	if la.NameSpace != getNameSpaceFromString(netwAttName) {
		fmt.Printf("NAMESPACE CHANGE: %v, %v \n", la.NameSpace,  getNameSpaceFromString(netwAttName))
		la.ChangeFlag = "configChange"
	}
	*/
}

// ValidateLinkCacheChanges function
func (c *workerController) ValidateLinkCacheChanges() error {
	fmt.Println("@@@@@@@@@@@@@@@@@@")
	for itfceName, v := range *c.linkCache {
		fmt.Printf("Link Cache: %s\n", itfceName)
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

			fmt.Printf("  VLAN: %d Attributes: %v\n", vlan, *la)
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
	fmt.Println("@@@@@@@@@@@@@@@@@@")
	return nil
}
