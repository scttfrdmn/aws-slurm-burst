package types

// InstanceInfo represents information about a launched AWS instance
type InstanceInfo struct {
	NodeName   string `json:"node_name"`
	InstanceID string `json:"instance_id"`
	PrivateIP  string `json:"private_ip"`
	PublicIP   string `json:"public_ip,omitempty"`
	State      string `json:"state"`
	LaunchTime string `json:"launch_time"`
}
