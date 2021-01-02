package fscagentctrlr

// Device is a struct that contains the information of a device element
type Device struct {
	Kind      string
	ID        string
	IDType    string
	Endpoints map[string]*Endpoint
}

// Endpoint is a struct that contains information of a link endpoint
type Endpoint struct {
	ID           string
	IDType       string
	MTU          int
	HardwareAddr string
	OperState    string
	NameSpace    string
	ParentIndex  int
	Vlans        map[int]*Vlan
}

// Vlan is a struct that contains information of the vlan
type Vlan struct {
	ID int
	ChangeFlag   string
}

// LinkAttr stuct
type LinkAttr struct {
	MTU            int
	HardwareAddr   string
	OperState      string
	NameSpace      string
	ParentIndex    int
	SlaveLinks     []string
	ChangeFlag     string
	LldpChangeFlag bool
}

// LldpEndPoint struct
type LldpEndPoint struct {
	MTU                  int
	HardwareAddr         string
	OperState            string
	NameSpace            string
	ParentIndex          int
	LldpPortID           string
	LldpPortIDType       string
	DeviceName           string
	DeviceKind           string
	DeviceID             string
	DeviceIDType         string
	ChangeFlag           string
	DeviceLinkChangeFlag bool
}
