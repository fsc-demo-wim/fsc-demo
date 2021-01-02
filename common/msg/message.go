package msg

type msgID int

const (
	// Shutdown to request to shutdown, expects ACK on respChan
	Shutdown msgID = iota
	// Ack acknowledges a request.
	Ack
	// TimerWork triggers some work activity
	TimerWork
	// SriovUpdate indicates an update of the sriov information
	SriovUpdate
	// SriovDelete indicates a delete of the sriov information
	SriovDelete
	// MultusUpdate indicates an update of the multus information
	MultusUpdate
	// MultusDelete indicates a delete of the multus information
	MultusDelete
	// WorkloadUpdate indicates an update of the workload information
	WorkloadUpdate
	// WorkloadDelete indicates a delete of the workload information
	WorkloadDelete
	// NodeTopologyUpdate indicates an update of the nodeTopology information
	NodeTopologyUpdate
	// NodeTopologyDelete indicates a delete of the nodeTopology information
	NodeTopologyDelete
	// NodeUpdate indicates an update of the node information
	NodeUpdate
	// NodeDelete indicates a delete of the node information
	NodeDelete
)

// CMsg Control message channel type
type CMsg struct {
	Type     msgID
	KeyValue interface{}
	RespChan chan *CMsg
}
