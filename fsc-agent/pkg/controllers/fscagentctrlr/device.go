package fscagentctrlr

import (
	"fmt"

	apiv1 "github.com/henderiw/fsc-lib-go/pkg/apis/fsc.henderiw.be/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *workerController) DeviceCentricView() error {
	for itfceName, v := range *c.lldpLinkCache {
		for vlan, lldpep := range v {
			if d, ok := (*c.Devices)[lldpep.DeviceName]; ok {
				if _, ok := d.Endpoints[itfceName]; ok {
					// interfacename already existed
					if _, ok := d.Endpoints[itfceName].Vlans[vlan]; ok {
						// vlan already existed
						var cf string
						if lldpep.DeviceLinkChangeFlag {
							cf = "someChange"
						} else {
							cf = "noChange"
						}
						d.Endpoints[itfceName].Vlans[vlan] = &Vlan{
							ID:         vlan,
							ChangeFlag: cf,
						}
						lldpep.DeviceLinkChangeFlag = false
					} else {
						// vlan did not exist
						d.Endpoints[itfceName].Vlans[vlan] = &Vlan{
							ID:         vlan,
							ChangeFlag: "newlyAdded",
						}
					}
				} else {
					// interfacename did not exist
					d.Endpoints[itfceName] = &Endpoint{
						ID:           lldpep.LldpPortID,
						IDType:       lldpep.LldpPortIDType,
						MTU:          lldpep.MTU,
						HardwareAddr: lldpep.HardwareAddr,
						OperState:    lldpep.OperState,
						ParentIndex:  lldpep.ParentIndex,
						Vlans:        make(map[int]*Vlan),
					}
					(*c.Devices)[lldpep.DeviceName].Endpoints[itfceName].Vlans[vlan] = &Vlan{
						ID:         vlan,
						ChangeFlag: "newlyAdded",
					}
				}

			} else {
				(*c.Devices)[lldpep.DeviceName] = &Device{
					Kind:      lldpep.DeviceKind,
					ID:        lldpep.DeviceID,
					IDType:    lldpep.DeviceIDType,
					Endpoints: make(map[string]*Endpoint),
				}
				(*c.Devices)[lldpep.DeviceName].Endpoints[itfceName] = &Endpoint{
					ID:           lldpep.LldpPortID,
					IDType:       lldpep.LldpPortIDType,
					MTU:          lldpep.MTU,
					HardwareAddr: lldpep.HardwareAddr,
					OperState:    lldpep.OperState,
					ParentIndex:  lldpep.ParentIndex,
					Vlans:        make(map[int]*Vlan),
				}
				(*c.Devices)[lldpep.DeviceName].Endpoints[itfceName].Vlans[vlan] = &Vlan{
					ID:         vlan,
					ChangeFlag: "newlyAdded",
				}
			}
		}

	}
	return nil
}

func (c *workerController) ValidateDeviceChanges() (bool, error) {
	fmt.Println("%%%%%%%%%%%%%%%%%%%%%%%%%%%%%")
	changed := false
	for devName, d := range *c.Devices {
		fmt.Printf("Device Cache: %s\n", devName)
		fmt.Printf("Device Cache Attributes: %v\n", *d)
		for ifName, ep := range d.Endpoints {
			fmt.Printf("    Interface Name: %s\n", ifName)
			fmt.Printf("    Interface Attr: %v\n", ep)
			for vlanID, vlan := range ep.Vlans {
				fmt.Printf("      VLAN ID: %d\n", vlanID)
				fmt.Printf("      VLAN Attr: %v\n", *vlan)
				switch vlan.ChangeFlag {
				case "":
					vlan.ChangeFlag = "toBeDeleted"
					changed = true
					break
				case "noChange":
					changed = false
					break
				default:
					changed = true
				}
				if vlan.ChangeFlag == "toBeDeleted" {
					delete((*c.Devices)[devName].Endpoints[ifName].Vlans, vlanID)
				}
				// reset the change flag for the next validation
				vlan.ChangeFlag = ""
			}
			if len((*c.Devices)[devName].Endpoints[ifName].Vlans) == 0 {
				delete((*c.Devices)[devName].Endpoints, ifName)
			}
		}

	}
	fmt.Println("%%%%%%%%%%%%%%%%%%%%%%%%%%%%%")
	return changed, nil
}

func (c *workerController) UpdateK8sNodeTopology() error {
	c.nodeTopo.Spec.Devices = make([]*apiv1.Device, 0)
	for devName, d := range *c.Devices {
		/*
			device := new(apiv1.Device)
			device.Name = devName
			device.Kind = d.Kind
			device.DeviceIdentifier = d.ID
			device.DeviceIdentifierType = d.IDType
		*/

		device := &apiv1.Device{
			Name:                  devName,
			Kind:                  d.Kind,
			DeviceIdentifier:     d.ID,
			DeviceIdentifierType: d.IDType,
			Endpoints:             make([]*apiv1.Endpoint, 0),
		}

		for ifName, ep := range d.Endpoints {
			lag := false
			if ep.ParentIndex != 0 {
				lag = true
			}
			/*
				endpoint := new(apiv1.Endpoint)
				endpoint.Name = ifName
				endpoint.InterfaceIdentifier = ep.ID
				endpoint.InterfaceIdentifierType = ep.IDType
				endpoint.MTU = ep.MTU
				endpoint.LAG = lag
			*/

			endpoint := &apiv1.Endpoint{
				Name:                    ifName,
				InterfaceIdentifier:     ep.ID,
				InterfaceIdentifierType: ep.IDType,
				MTU:                     ep.MTU,
				LAG:                     lag,
				Vlans:                   make([]*apiv1.Vlan, 0),
			}

			for vlanID := range ep.Vlans {
				/*
					vlan := new(apiv1.Vlan)
					vlan.ID = vlanID
				*/

				vlan := &apiv1.Vlan{
					ID: vlanID,
				}

				endpoint.Vlans = append(endpoint.Vlans, vlan)
			}
			device.Endpoints = append(device.Endpoints, endpoint)
		}
		c.nodeTopo.Spec.Devices = append(c.nodeTopo.Spec.Devices, device)
	}
	c.nodeTopo.DeepCopy()
	c.showNodeTopology()

	var err error
	c.nodeTopo, err = c.nti.Update(c.ctx, c.nodeTopo, metav1.UpdateOptions{})
	c.nodeTopo.DeepCopy()
	if err != nil {
		return err
	}
	return nil
}

func (c *workerController) showNodeTopology() {
	for _, d := range c.nodeTopo.Spec.Devices {
		fmt.Printf("Devices: %#v\n", d)
		for _, ep := range d.Endpoints {
			fmt.Printf("Endpoints: %#v\n", ep)
			for _, vlan := range ep.Vlans {
				fmt.Printf("Vlans: %#v\n", vlan)
			}
		}
	}
}
