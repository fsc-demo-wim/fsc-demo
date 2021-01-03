package fscctrlctrlr

import (
	"github.com/google/go-cmp/cmp"
	log "github.com/sirupsen/logrus"

	apiv1 "github.com/fsc-demo-wim/fsc-lib-go/pkg/apis/fsc.henderiw.be/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *workerController) constructWorkloadUpdates() map[string]*apiv1.WorkLoadStatus {
	newWorkloadStatus := make(map[string]*apiv1.WorkLoadStatus)
	for wlName, wl := range c.WorkLoad {
		newWorkloadStatus[wlName] = new(apiv1.WorkLoadStatus)
		newWorkloadStatus[wlName].Devices = make([]*apiv1.Device, 0)

		// a workload could have multiple vlans
		idxNewDev := 0
		idxNewEpPerDev := make(map[int]int) // keeps track of the endpoints per device index
		for _, vl := range wl.Spec.Vlans {
			// pick up vlans in the node topology
			for _, nt := range c.NodeTopology {
				for _, d := range nt.Spec.Devices {
					for _, ntep := range d.Endpoints {
						for _, ntv := range ntep.Vlans {
							if vl.ID == ntv.ID {
								newWorkloadStatus[wlName].RunningConfig = wl.Spec
								devFound := false
								for idxCurdev, dev := range newWorkloadStatus[wlName].Devices {
									if d.Name == dev.Name {
										// device is found already
										devFound = true
										epFound := false
										for _, endp := range dev.Endpoints {
											if ntep.InterfaceIdentifierType == endp.InterfaceIdentifier && ntep.Name == endp.Name {
												epFound = true
												vlanFound := false
												for _, vlan := range endp.Vlans {
													if ntv.ID == vlan.ID {
														vlanFound = true
													}
												}
												if !vlanFound {
													log.Debugf("New VLAN dName %s, epName %s, vlanID %d %d %d %d\n", d.Name, ntep.Name, ntv.ID, idxCurdev, idxNewDev, idxNewEpPerDev[idxCurdev])
													vlan := &apiv1.Vlan{
														ID: ntv.ID,
													}
													newWorkloadStatus[wlName].Devices[idxCurdev].Endpoints[idxNewEpPerDev[idxCurdev]].Vlans = append(newWorkloadStatus[wlName].Devices[idxCurdev].Endpoints[idxNewEpPerDev[idxCurdev]].Vlans, vlan)
												}
											}
										}
										if !epFound {
											log.Debugf("New EP dName %s, epName %s, vlanID %d %d %d %d\n", d.Name, ntep.Name, ntv.ID, idxCurdev, idxNewDev, idxNewEpPerDev[idxCurdev])
											// create a new endpoint and vlan
											endpoint := &apiv1.Endpoint{
												Name:                    ntep.Name,
												InterfaceIdentifier:     ntep.InterfaceIdentifier,
												InterfaceIdentifierType: ntep.InterfaceIdentifierType,
												MTU:                     ntep.MTU,
												LAG:                     ntep.LAG,
												Vlans:                   make([]*apiv1.Vlan, 0),
											}
											newWorkloadStatus[wlName].Devices[idxCurdev].Endpoints = append(newWorkloadStatus[wlName].Devices[idxCurdev].Endpoints, endpoint)
											vlan := &apiv1.Vlan{
												ID: ntv.ID,
											}
											newWorkloadStatus[wlName].Devices[idxCurdev].Endpoints[idxNewEpPerDev[idxCurdev]].Vlans = append(newWorkloadStatus[wlName].Devices[idxCurdev].Endpoints[idxNewEpPerDev[idxCurdev]].Vlans, vlan)
											idxNewEpPerDev[idxCurdev]++
										}
									}
								}
								if !devFound {
									log.Debugf("New Switch dName %s, epName %s, vlanID %d %d\n", d.Name, ntep.Name, ntv.ID, idxNewDev)
									vlan := &apiv1.Vlan{
										ID: ntv.ID,
									}
									endpoint := &apiv1.Endpoint{
										Name:                    ntep.Name,
										InterfaceIdentifier:     ntep.InterfaceIdentifier,
										InterfaceIdentifierType: ntep.InterfaceIdentifierType,
										MTU:                     ntep.MTU,
										LAG:                     ntep.LAG,
										Vlans:                   make([]*apiv1.Vlan, 0),
									}
									device := &apiv1.Device{
										Name:                 d.Name,
										Kind:                 d.Kind,
										DeviceIdentifier:     d.DeviceIdentifier,
										DeviceIdentifierType: d.DeviceIdentifierType,
										Endpoints:            make([]*apiv1.Endpoint, 0),
									}
									idxNewEpPerDev[idxNewDev] = 0
									newWorkloadStatus[wlName].Devices = append(newWorkloadStatus[wlName].Devices, device)
									newWorkloadStatus[wlName].Devices[idxNewDev].Endpoints = append(newWorkloadStatus[wlName].Devices[idxNewDev].Endpoints, endpoint)
									newWorkloadStatus[wlName].Devices[idxNewDev].Endpoints[idxNewEpPerDev[idxNewDev]].Vlans = append(newWorkloadStatus[wlName].Devices[idxNewDev].Endpoints[idxNewEpPerDev[idxNewDev]].Vlans, vlan)
									idxNewEpPerDev[idxNewDev]++
									idxNewDev++

								}
							}
						}
					}
				}
			}
		}
	}
	return newWorkloadStatus
}

func (c *workerController) compareWorkloadStatus(newWorkLoadStatus map[string]*apiv1.WorkLoadStatus) {
	workLoadChangeFlag := make(map[string]bool)

	for wlName, newWl := range newWorkLoadStatus {
		var err error
		if curWl, ok := c.WorkLoadStatus[wlName]; ok {
			// worload exists
			change := false
			if !cmp.Equal(newWl, curWl) {
				change = true
			}
			if change {
				// UPDATE WORKLOAD STATUS to K8S FSC API
				c.WorkLoad[wlName].Status = *newWorkLoadStatus[wlName]
				c.WorkLoad[wlName].Status.DeepCopy()
				c.WorkLoad[wlName].DeepCopy()

				log.Debugf("Workload name: %v\n", c.WorkLoad[wlName].Name)
				log.Debugf("Workload namespace: %v\n", c.WorkLoad[wlName].Namespace)
				log.Debugf("Workload spec  config: %v\n", c.WorkLoad[wlName].Spec)
				log.Debugf("Workload status running config: %v\n", c.WorkLoad[wlName].Status.RunningConfig)
				for _, device := range c.WorkLoad[wlName].Status.Devices {
					log.Debugf("Workload status devices: %v\n", device)
				}
				c.WorkLoad[wlName], err = c.wli.Update(c.ctx, c.WorkLoad[wlName], metav1.UpdateOptions{})
				c.WorkLoad[wlName].DeepCopy()
				if err != nil {
					log.Error(err)
				}

				log.Info("update workload status to the k8s fsc api")
				workLoadChangeFlag[wlName] = true
			}
			// No UPDATE REQUIRED
			log.Info("No workload status update required to the k8s fsc api")
			workLoadChangeFlag[wlName] = false
		} else {
			// new workload
			workLoadChangeFlag[wlName] = true
			// UPDATE WORKLOAD STATUS to K8S FSC API
			c.WorkLoad[wlName].Status = *newWorkLoadStatus[wlName]
			c.WorkLoad[wlName].Status.DeepCopy()
			c.WorkLoad[wlName].DeepCopy()

			log.Debugf("Workload name: %v\n", c.WorkLoad[wlName].Name)
			log.Debugf("Workload namespace: %v\n", c.WorkLoad[wlName].Namespace)
			log.Debugf("Workload spec  config: %v\n", c.WorkLoad[wlName].Spec)
			log.Debugf("Workload status running config: %v\n", c.WorkLoad[wlName].Status.RunningConfig)
			for _, device := range c.WorkLoad[wlName].Status.Devices {
				log.Debugf("Workload status devices: %v\n", device)
			}
			c.WorkLoad[wlName], err = c.wli.Update(c.ctx, c.WorkLoad[wlName], metav1.UpdateOptions{})
			c.WorkLoad[wlName].DeepCopy()
			if err != nil {
				log.Error(err)
			}

			log.Info("update workload status to the k8s fsc api")
		}
	}
	// check if a workload got deleted
	for wlName := range c.WorkLoadStatus {
		if _, ok := workLoadChangeFlag[wlName]; !ok {
			// deleted workload
			// The Delete should have happpend already so this can be avoided
			log.Info("delete workload status to the k8s fsc api")
		}
	}
	c.WorkLoadStatus = newWorkLoadStatus
}

func (c *workerController) showWorkLoads() {
	for wlName, wls := range c.WorkLoadStatus {
		log.Debugf("Workload Name: %s\n", wlName)
		log.Debugf("  Running Config: %v\n", wls.RunningConfig)
		for _, d := range wls.Devices {
			log.Debugf("  Device Name %s\n", d.Name)
			log.Debugf("  Device Attaributes %v\n", *d)
			for _, ep := range d.Endpoints {
				log.Debugf("    Endpoint Name %s\n", ep.Name)
				log.Debugf("    Endpoint InterfaceIdentifier %s\n", ep.InterfaceIdentifier)
				log.Debugf("    Endpoint Attributes %v\n", *ep)
				for _, v := range ep.Vlans {
					log.Debugf("    Vlan ID %d\n", v.ID)
				}
			}
		}
	}
}
