package fscagentctrlr

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Discovery struct
type Discovery struct {
	Lldp Lldp `json:"lldp,omitempty"`
}

// Lldp struct
type Lldp struct {
	Itfce []map[string]Itfce `json:"interface,omitempty"`
}

// Itfce struct
type Itfce struct {
	Via     string             `json:"via,omitempty"`
	Rid     string             `json:"rid,omitempty"`
	Age     string             `json:"age,omitempty"`
	Chassis map[string]Chassis `json:"chassis,omitempty"`
	Port    Port               `json:"port,omitempty"`
}

// Chassis struct
type Chassis struct {
	ID    ID     `json:"id,omitempty"`
	Descr string `json:"descr,omitempty"`
	//MgmtIP     []string     `json:"mgmt-ip,omitempty"`
	Capability []Capability `json:"capability,omitempty"`
}

// Capability struct
type Capability struct {
	Type    string `json:"type,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
}

// Port struct
type Port struct {
	ID    ID     `json:"id,omitempty"`
	Descr string `json:"descr,omitempty"`
	TTL   string `json:"ttl,omitempty"`
}

// ID struct
type ID struct {
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

// CheckLldpDaemon function
func CheckLldpDaemon() error {
	//cmd := exec.Command("systemctl", "check", "lldpd")
	cmd := exec.Command("lldpcli", "show", "neighbor")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("systemctl finished with non-zero: %v, status: %s", exitErr, string(out))
		}
	}
	log.Infof("LLDP Systemctl Status is: %s", string(out))
	return nil
}

// GetLldpTopology function
func (c *workerController) getLldpTopology() (*Discovery, error) {
	cmd := exec.Command("lldpcli", "show", "neighbors", "-f", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("lldpcli finished with non-zero: %v", exitErr)
		}
		return nil, err
	}
	log.Infof("Status is: %s\n", string(out))

	var d Discovery
	err = json.Unmarshal(out, &d)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

func (c *workerController) UpdateLldpLinkCache() error {
	lldpDisc, err := c.getLldpTopology()
	if err != nil {
		log.Errorf("Get LLDP topology failure: %s", err)
	}
	// loop over the cache to see if we have a matching interface
	// with the LLDP topology discovery
	for _, v := range *c.linkCache {
		for vlan, la := range v {
			// we copied slavelinks also for non bonded interfaces,
			// so we can just loop over that
			for _, ifName := range la.SlaveLinks {
				// loop over the lldp discovery
				for _, lldpitfces := range lldpDisc.Lldp.Itfce {
					for lldpifName, lldpitfce := range lldpitfces {
						// we only care about interfaces that are discovered via multus
						if lldpifName == ifName {
							// link found
							if clldpl, ok := (*c.lldpLinkCache)[lldpifName]; ok {
								// link exists in lldp cache
								for dName, ch := range lldpitfce.Chassis {
									var cf string
									var dcf bool
									if la.LldpChangeFlag {
										cf = "someChange"
										dcf = true
									} else {
										cf = "noChange"
										dcf = false
									}
									clldpl[vlan] = &LldpEndPoint{
										ChangeFlag:           cf,
										MTU:                  la.MTU,
										HardwareAddr:         la.HardwareAddr,
										OperState:            la.OperState,
										NameSpace:            la.NameSpace,
										ParentIndex:          la.ParentIndex,
										DeviceName:           dName,
										DeviceKind:           findDeviceKind(&ch.Descr),
										DeviceID:             ch.ID.Value,
										DeviceIDType:         ch.ID.Type,
										LldpPortID:           lldpitfce.Port.ID.Value,
										LldpPortIDType:       lldpitfce.Port.ID.Type,
										DeviceLinkChangeFlag: dcf,
									}
									// reset the change flag
									la.LldpChangeFlag = false
								}
							} else {
								// link does not exists in lldp cache
								// creare a new link and initialize the LldpEndPoint map
								(*c.lldpLinkCache)[lldpifName] = make(map[int]*LldpEndPoint)
								// there should only be one device connected to a link
								for dName, ch := range lldpitfce.Chassis {
									(*c.lldpLinkCache)[lldpifName][vlan] = &LldpEndPoint{
										ChangeFlag:           "newlyAdded",
										MTU:                  la.MTU,
										HardwareAddr:         la.HardwareAddr,
										OperState:            la.OperState,
										NameSpace:            la.NameSpace,
										ParentIndex:          la.ParentIndex,
										DeviceName:           dName,
										DeviceKind:           findDeviceKind(&ch.Descr),
										DeviceID:             ch.ID.Value,
										DeviceIDType:         ch.ID.Type,
										LldpPortID:           lldpitfce.Port.ID.Value,
										LldpPortIDType:       lldpitfce.Port.ID.Type,
										DeviceLinkChangeFlag: true,
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func (c *workerController) ValidateLldpLinkCacheChanges() error {
	log.Info("ValidateLldpLinkCacheChanges start ...")
	for itfceName, v := range *c.lldpLinkCache {
		log.Infof("LLDP Link Cache: %s\n", itfceName)
		for vlan, lldpep := range v {
			// validate change flag
			// when empty: "" -> the link no longer exists
			// for others we indicate the change in LldpChangeFlag flag to indicate
			// changes to the next processing level
			switch lldpep.ChangeFlag {
			case "":
				lldpep.ChangeFlag = "toBeDeleted"
				break
			}
			log.Infof("  VLAN: %d Attributes: %v\n", vlan, *lldpep)
			// if the vlan no longer exists delete it from the cache
			if lldpep.ChangeFlag == "toBeDeleted" {
				delete((*c.lldpLinkCache)[itfceName], vlan)
			}
			lldpep.ChangeFlag = ""
		}
		// if there are no vlans only longer on the links delete the link
		if len((*c.lldpLinkCache)[itfceName]) == 0 {
			delete(*c.lldpLinkCache, itfceName)
		}
	}
	log.Info("ValidateLldpLinkCacheChanges stop ...")
	return nil
}

func findDeviceKind(d *string) string {
	if strings.Contains(*d, "TiMOS") && strings.Contains(*d, "NUAGE") {
		return "wbx"
	}
	if strings.Contains(*d, "SRLinux") {
		return "srl"
	}
	if strings.Contains(*d, "Ubuntu") {
		return "linux"
	}
	if strings.Contains(*d, "Centos") {
		return "linux"
	}
	if strings.Contains(*d, "Redhat") {
		return "linux"
	}
	return "linux"
}
