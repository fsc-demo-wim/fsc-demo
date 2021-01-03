# fsc-demo
 
The goal of this app is to automate the configuration of multus netwoks beyond the cluster. The app extends the cluster networking configuration to the atached dc switches such that we can provide end to end connectivity from cluster to outside world. The application consists of 2 functions:
- An agent (fsc-agent) that is deployed as a daemonset and provides the nodetopology aligned with network connectivity/topology and multus configuration/usage
- A controller that leverages the node-topology provided through the agent and use this to augment the workload CRD with the switches, interfaces, vlans information that are active in the cluster for multus networking.

## fsc-agent

Deployed as a daemonset. Listens to the following information:
- LLDP
- Netlink
- Multus Network attachement
- Configmap for sriov confguration

Based on this information it constructs a node-topology CRD, which indicates the interfaces and vlans that could be of interest for automation by the fsc-controller.

## fsc-controller

Deployed in K8s as a controller that listens to the workloads, nodetopologies (constructed by the fsc-agent) and nodes that are deployed in the K8s cluster.
The workload CRD provide the networking parameters that are used for multus networking, beyond vlans, such as:

- RT
- RD
- VLANs
- Network Policies
- QoS
- etc

Based on nodes, node topologies and workload information a switch configuration is constrcuted that aligns the switch configuration with the cluster application for multus networking.
With this information the cluster can be connected to the outside world for connectivity

## installation pre-requisites

Firstly, the cluster should be enabled for [sriov and multus](https://github.com/intel/multus-cni)

Second we expect the following daemons to be installed, which provide access wrt lldp and netlink information to the fsc-agent

- llpdd 

```
sudo apt-get install lldpd
```

- netlinkd

see [netlinkd](https://github.com/fsc-demo-wim/netlinkd)
 

## installation

Installation of the fsc-demo app is provided theough helm charts using the following repo:

- [fsc-demo-charts repo](https://github.com/fsc-demo-wim/fsc-demo-charts)



