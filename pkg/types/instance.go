package types

// InstanceRequirements defines what type of instance is needed for a job
type InstanceRequirements struct {
	// Basic compute requirements
	MinCPUs      int    `json:"min_cpus"`
	MinMemoryMB  int    `json:"min_memory_mb"`
	GPUs         int    `json:"gpus,omitempty"`
	GPUType      string `json:"gpu_type,omitempty"`

	// Network requirements
	RequiresEFA         bool            `json:"requires_efa"`
	EFAPreferred        bool            `json:"efa_preferred"`
	NetworkTopology     NetworkTopology `json:"network_topology"`
	PlacementGroupType  string          `json:"placement_group_type,omitempty"`

	// Instance preferences
	InstanceFamilies    []string `json:"instance_families,omitempty"`    // ["c5n", "c6i", "hpc6a"]
	ExcludeInstances    []string `json:"exclude_instances,omitempty"`    // Specific instances to avoid
	HPCOptimized        bool     `json:"hpc_optimized"`                  // Prefer HPC-optimized instances
	EnhancedNetworking  bool     `json:"enhanced_networking"`            // Requires SR-IOV

	// Cost constraints
	MaxSpotPrice       float64 `json:"max_spot_price,omitempty"`
	PreferSpot         bool    `json:"prefer_spot"`
	AllowMixedPricing  bool    `json:"allow_mixed_pricing"`            // Mix spot/on-demand
}

// EFACapability represents EFA support levels
type EFACapability string

const (
	EFARequired  EFACapability = "required"   // Job fails without EFA
	EFAPreferred EFACapability = "preferred"  // Better performance with EFA
	EFAOptional  EFACapability = "optional"   // Can use EFA if available
	EFADisabled  EFACapability = "disabled"   // Don't use EFA
)

// InstanceFamily represents AWS instance family characteristics
type InstanceFamily struct {
	Name                string   `json:"name"`                // "c5n", "hpc6a", etc.
	SupportsEFA         bool     `json:"supports_efa"`
	EFAGeneration      int      `json:"efa_generation"`      // 1 or 2
	NetworkPerformance string   `json:"network_performance"` // "Up to 100 Gbps"
	HPCOptimized       bool     `json:"hpc_optimized"`
	AvailableRegions   []string `json:"available_regions"`
}

// HPC-Optimized Instance Types with EFA Support
var HPCInstanceFamilies = map[string]InstanceFamily{
	"hpc6a": {
		Name:                "hpc6a",
		SupportsEFA:         true,
		EFAGeneration:      2,
		NetworkPerformance:  "100 Gbps",
		HPCOptimized:       true,
		AvailableRegions:   []string{"us-east-1", "us-west-2", "eu-west-1"},
	},
	"hpc6id": {
		Name:                "hpc6id",
		SupportsEFA:         true,
		EFAGeneration:      2,
		NetworkPerformance:  "200 Gbps",
		HPCOptimized:       true,
		AvailableRegions:   []string{"us-east-1", "us-west-2"},
	},
	"hpc7a": {
		Name:                "hpc7a",
		SupportsEFA:         true,
		EFAGeneration:      2,
		NetworkPerformance:  "300 Gbps",
		HPCOptimized:       true,
		AvailableRegions:   []string{"us-east-1", "us-west-2"},
	},
}

// Compute-Optimized with EFA Support
var ComputeEFAFamilies = map[string]InstanceFamily{
	"c5n": {
		Name:                "c5n",
		SupportsEFA:         true,
		EFAGeneration:      1,
		NetworkPerformance:  "Up to 100 Gbps",
		HPCOptimized:       false,
		AvailableRegions:   []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-2"},
	},
	"c6i": {
		Name:                "c6i",
		SupportsEFA:         true,
		EFAGeneration:      2,
		NetworkPerformance:  "Up to 50 Gbps",
		HPCOptimized:       false,
		AvailableRegions:   []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"},
	},
	"c6in": {
		Name:                "c6in",
		SupportsEFA:         true,
		EFAGeneration:      2,
		NetworkPerformance:  "200 Gbps",
		HPCOptimized:       false,
		AvailableRegions:   []string{"us-east-1", "us-west-2", "eu-west-1"},
	},
}

// Memory-Optimized with EFA Support
var MemoryEFAFamilies = map[string]InstanceFamily{
	"r5n": {
		Name:                "r5n",
		SupportsEFA:         true,
		EFAGeneration:      1,
		NetworkPerformance:  "Up to 100 Gbps",
		HPCOptimized:       false,
		AvailableRegions:   []string{"us-east-1", "us-west-2", "eu-west-1"},
	},
	"r6i": {
		Name:                "r6i",
		SupportsEFA:         true,
		EFAGeneration:      2,
		NetworkPerformance:  "Up to 50 Gbps",
		HPCOptimized:       false,
		AvailableRegions:   []string{"us-east-1", "us-west-2", "eu-west-1"},
	},
}

// GetEFASupportedFamilies returns all instance families that support EFA
func GetEFASupportedFamilies() []InstanceFamily {
	var families []InstanceFamily

	for _, family := range HPCInstanceFamilies {
		families = append(families, family)
	}
	for _, family := range ComputeEFAFamilies {
		families = append(families, family)
	}
	for _, family := range MemoryEFAFamilies {
		families = append(families, family)
	}

	return families
}

// IsEFASupported checks if an instance family supports EFA
func IsEFASupported(instanceFamily string) (bool, int) {
	allFamilies := map[string]InstanceFamily{}

	// Merge all family maps
	for k, v := range HPCInstanceFamilies {
		allFamilies[k] = v
	}
	for k, v := range ComputeEFAFamilies {
		allFamilies[k] = v
	}
	for k, v := range MemoryEFAFamilies {
		allFamilies[k] = v
	}

	if family, exists := allFamilies[instanceFamily]; exists {
		return family.SupportsEFA, family.EFAGeneration
	}

	return false, 0
}