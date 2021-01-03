package fscagentctrlr

import (
	apiv1 "github.com/fsc-demo-wim/fsc-lib-go/pkg/apis/fsc.henderiw.be/v1"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeviceCentricView function provides a view on the connectivity from the host to the rest of the network.
// The function also provides a view on the changes that are relevant for the multus networking
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
	log.Infof("ValidateDeviceChanges Start...")
	changed := false
	for devName, d := range *c.Devices {
		log.Debugf("Device Cache: %s\n", devName)
		log.Debugf("Device Cache Attributes: %v\n", *d)
		for ifName, ep := range d.Endpoints {
			log.Debugf("    Interface Name: %s\n", ifName)
			log.Debugf("    Interface Attr: %v\n", ep)
			for vlanID, vlan := range ep.Vlans {
				log.Debugf("      VLAN ID: %d\n", vlanID)
				log.Debugf("      VLAN Attr: %v\n", *vlan)
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
	log.Info("ValidateDeviceChanges End...")
	return changed, nil
}

func (c *workerController) UpdateK8sNodeTopology() error {
	c.nodeTopo.Spec.Devices = make([]*apiv1.Device, 0)
	for devName, d := range *c.Devices {
		device := &apiv1.Device{
			Name:                 devName,
			Kind:                 d.Kind,
			DeviceIdentifier:     d.ID,
			DeviceIdentifierType: d.IDType,
			Endpoints:            make([]*apiv1.Endpoint, 0),
		}

		for ifName, ep := range d.Endpoints {
			lag := false
			if ep.ParentIndex != 0 {
				lag = true
			}
			endpoint := &apiv1.Endpoint{
				Name:                    ifName,
				InterfaceIdentifier:     ep.ID,
				InterfaceIdentifierType: ep.IDType,
				MTU:                     ep.MTU,
				LAG:                     lag,
				Vlans:                   make([]*apiv1.Vlan, 0),
			}

			for vlanID := range ep.Vlans {
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
		log.Infof("Devices: %#v", d)
		for _, ep := range d.Endpoints {
			log.Infof("Endpoints: %#v", ep)
			for _, vlan := range ep.Vlans {
				log.Infof("Vlans: %#v", vlan)
			}
		}
	}
}
