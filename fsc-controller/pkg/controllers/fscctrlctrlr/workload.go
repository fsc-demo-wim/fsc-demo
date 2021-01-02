package fscctrlctrlr

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
	"k8s.io/klog"

	apiv1 "github.com/henderiw/fsc-lib-go/pkg/apis/fsc.henderiw.be/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *workerController) constructWorkloadUpdates() map[string]*apiv1.WorkLoadStatus {
	newWorkloadStatus := make(map[string]*apiv1.WorkLoadStatus)
	for wlName, wl := range c.WorkLoad {
		newWorkloadStatus[wlName] = new(apiv1.WorkLoadStatus)
		newWorkloadStatus[wlName].Devices = make([]*apiv1.Device, 0)
		idxdev := 0
		idxdev2 := 0
		idxep := 0
		//idxep2 := 0
		for _, vl := range wl.Spec.Vlans {
			// pick up vlans in the node topology
			for _, nt := range c.NodeTopology {
				for _, d := range nt.Spec.Devices {
					for _, ep := range d.Endpoints {
						for _, v := range ep.Vlans {
							if vl.ID == v.ID {
								newWorkloadStatus[wlName].RunningConfig = wl.Spec
								devFound := false
								for _, dev := range newWorkloadStatus[wlName].Devices {
									if d.Name == dev.Name {
										// device is found already
										devFound = true
										epFound := false
										for _, endp := range dev.Endpoints {
											if ep.Name == endp.Name {
												epFound = true
												vlanFound := false
												for _, vlan := range endp.Vlans {
													if v.ID == vlan.ID {
														vlanFound = true
													}
												}
												if !vlanFound {
													fmt.Printf("New VLAN dName %s, epName %s, vlanID %d %d %d\n", d.Name, ep.Name, v.ID, idxdev, idxep)
													vlan := &apiv1.Vlan{
														ID: v.ID,
													}
													newWorkloadStatus[wlName].Devices[idxdev].Endpoints[idxep].Vlans = append(newWorkloadStatus[wlName].Devices[idxdev].Endpoints[idxep].Vlans, vlan)
												}
											}
										}
										if !epFound {
											fmt.Printf("New EP dName %s, epName %s, vlanID %d %d %d\n", d.Name, ep.Name, v.ID, idxdev, idxep)
											// create a new endpoint and vlan
											endpoint := &apiv1.Endpoint{
												Name:                    ep.Name,
												InterfaceIdentifier:     ep.InterfaceIdentifier,
												InterfaceIdentifierType: ep.InterfaceIdentifierType,
												MTU:                     ep.MTU,
												LAG:                     ep.LAG,
												Vlans:                   make([]*apiv1.Vlan, 0),
											}
											newWorkloadStatus[wlName].Devices[idxdev].Endpoints = append(newWorkloadStatus[wlName].Devices[idxdev].Endpoints, endpoint)
											vlan := &apiv1.Vlan{
												ID: v.ID,
											}
											newWorkloadStatus[wlName].Devices[idxdev].Endpoints[idxep].Vlans = append(newWorkloadStatus[wlName].Devices[idxdev].Endpoints[idxep].Vlans, vlan)
											idxep++
										}
									}
								}
								if !devFound {
									fmt.Printf("New Switch dName %s, epName %s, vlanID %d %d\n", d.Name, ep.Name, v.ID, idxdev)
									vlan := &apiv1.Vlan{
										ID: v.ID,
									}
									endpoint := &apiv1.Endpoint{
										Name:                    ep.Name,
										InterfaceIdentifier:     ep.InterfaceIdentifier,
										InterfaceIdentifierType: ep.InterfaceIdentifierType,
										MTU:                     ep.MTU,
										LAG:                     ep.LAG,
										Vlans:                   make([]*apiv1.Vlan, 0),
									}
									device := &apiv1.Device{
										Name:                 d.Name,
										Kind:                 d.Kind,
										DeviceIdentifier:     d.DeviceIdentifier,
										DeviceIdentifierType: d.DeviceIdentifierType,
										Endpoints:            make([]*apiv1.Endpoint, 0),
									}
									idxep = 0
									idxdev = idxdev2
									newWorkloadStatus[wlName].Devices = append(newWorkloadStatus[wlName].Devices, device)
									newWorkloadStatus[wlName].Devices[idxdev].Endpoints = append(newWorkloadStatus[wlName].Devices[idxdev].Endpoints, endpoint)
									newWorkloadStatus[wlName].Devices[idxdev].Endpoints[idxep].Vlans = append(newWorkloadStatus[wlName].Devices[idxdev].Endpoints[idxep].Vlans, vlan)
									idxdev2++
									idxep++
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

				fmt.Printf("Workload name: %v\n", c.WorkLoad[wlName].Name)
				fmt.Printf("Workload namespace: %v\n", c.WorkLoad[wlName].Namespace)
				fmt.Printf("Workload spec  config: %v\n", c.WorkLoad[wlName].Spec)
				fmt.Printf("Workload status running config: %v\n", c.WorkLoad[wlName].Status.RunningConfig)
				for _, device := range c.WorkLoad[wlName].Status.Devices {
					fmt.Printf("Workload status devices: %v\n", device)
				}
				c.WorkLoad[wlName], err = c.wli.Update(c.ctx, c.WorkLoad[wlName], metav1.UpdateOptions{})
				c.WorkLoad[wlName].DeepCopy()
				if err != nil {
					klog.Error(err)
				}

				klog.Info("update workload status to the k8s fsc api")
				workLoadChangeFlag[wlName] = true
			}
			// No UPDATE REQUIRED
			klog.Info("No workload status update required to the k8s fsc api")
			workLoadChangeFlag[wlName] = false
		} else {
			// new workload
			workLoadChangeFlag[wlName] = true
			// UPDATE WORKLOAD STATUS to K8S FSC API
			c.WorkLoad[wlName].Status = *newWorkLoadStatus[wlName]
			c.WorkLoad[wlName].Status.DeepCopy()
			c.WorkLoad[wlName].DeepCopy()

			fmt.Printf("Workload name: %v\n", c.WorkLoad[wlName].Name)
			fmt.Printf("Workload namespace: %v\n", c.WorkLoad[wlName].Namespace)
			fmt.Printf("Workload spec  config: %v\n", c.WorkLoad[wlName].Spec)
			fmt.Printf("Workload status running config: %v\n", c.WorkLoad[wlName].Status.RunningConfig)
			for _, device := range c.WorkLoad[wlName].Status.Devices {
				fmt.Printf("Workload status devices: %v\n", device)
			}
			c.WorkLoad[wlName], err = c.wli.Update(c.ctx, c.WorkLoad[wlName], metav1.UpdateOptions{})
			c.WorkLoad[wlName].DeepCopy()
			if err != nil {
				klog.Error(err)
			}

			klog.Info("update workload status to the k8s fsc api")
		}
	}
	// check if a workload got deleted
	for wlName := range c.WorkLoadStatus {
		if _, ok := workLoadChangeFlag[wlName]; !ok {
			// deleted workload
			// The Delete should have happpend already so this can be avoided
			klog.Info("delete workload status to the k8s fsc api")
		}
	}
	c.WorkLoadStatus = newWorkLoadStatus
}

func (c *workerController) showWorkLoads() {
	for wlName, wls := range c.WorkLoadStatus {
		fmt.Printf("Workload Name: %s\n", wlName)
		fmt.Printf("  Running Config: %v\n", wls.RunningConfig)
		for _, d := range wls.Devices {
			fmt.Printf("  Device Name %s\n", d.Name)
			fmt.Printf("  Device Attaributes %v\n", *d)
			for _, ep := range d.Endpoints {
				fmt.Printf("    Endpoint Name %s\n", ep.Name)
				fmt.Printf("    Endpoint Attaributes %v\n", *ep)
				for _, v := range ep.Vlans {
					fmt.Printf("    Vlan ID %d\n", v.ID)
				}
			}
		}
	}
}
