package main

type ResourceSpec struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	Cores        int     `json:"cores"`
	Memory       float64 `json:"memory"`
	Bandwidth    float64 `json:"bandwidth"`
	BootOverhead float64 `json:"boot_overhead"`
	Location     string  `json:"location"`
}

type SimulationRequest struct {
	Preset          string         `json:"preset"`
	Seed            int64          `json:"seed"`
	TaskCount       int            `json:"task_count"`
	EdgeDensity     float64        `json:"edge_density"`
	ClusterMachines int            `json:"cluster_machines"`
	CloudMachines   int            `json:"cloud_machines"`
	CoresPerMachine int            `json:"cores_per_machine"`
	WeightTime      float64        `json:"weight_time"`
	WeightCost      float64        `json:"weight_cost"`
	BudgetLimit     *float64       `json:"budget_limit"`
	DeadlineLimit   *float64       `json:"deadline_limit"`
	OptionCount     int            `json:"option_count"`
	BeamWidth       int            `json:"beam_width"`
	WorkflowYAML    *string        `json:"workflow_yaml"`
	ResourceSpecs   []ResourceSpec `json:"resource_specs"`
}

type Core struct {
	ID    string  `json:"id"`
	Index int     `json:"index"`
	Avail float64 `json:"avail"`
}

type Resource struct {
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Kind                  string   `json:"kind"`
	Cores                 []Core   `json:"cores"`
	CPU                   float64  `json:"cpu"`
	Memory                float64  `json:"memory"`
	PricePerCPUSecond     float64  `json:"price_per_cpu_second"`
	PricePerGBSecond      float64  `json:"price_per_gb_second"`
	FinancialNetworkPrice float64  `json:"financial_network_price"`
	Bandwidth             float64  `json:"bandwidth"`
	Location              string   `json:"location"`
	Status                string   `json:"status"`
	BootOverhead          float64  `json:"boot_overhead"`
	ImageCache            []string `json:"image_cache"`
}

type Task struct {
	ID            string   `json:"id"`
	Label         string   `json:"label"`
	WorkflowStage string   `json:"workflow_stage"`
	CPU           float64  `json:"cpu"`
	Memory        float64  `json:"memory"`
	BaseRuntime   float64  `json:"base_runtime"`
	Image         string   `json:"image"`
	Runtime       *string  `json:"runtime"`
	Run           *string  `json:"run"`
	Predecessors  []string `json:"predecessors"`
	Successors    []string `json:"successors"`
}

type Dependency struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	DataMB float64 `json:"data_mb"`
}

type SLA struct {
	WeightTime    float64  `json:"weight_time"`
	WeightCost    float64  `json:"weight_cost"`
	BudgetLimit   *float64 `json:"budget_limit"`
	DeadlineLimit *float64 `json:"deadline_limit"`
	OptionCount   int      `json:"option_count"`
	BeamWidth     int      `json:"beam_width"`
}

type Workflow struct {
	Preset          string              `json:"preset"`
	Tasks           []Task              `json:"tasks"`
	Dependencies    []Dependency        `json:"dependencies"`
	PredecessorSets map[string][]string `json:"predecessor_sets"`
}

type Matrices struct {
	ET0                  map[string]map[string]float64                       `json:"et_0"`
	ETStar               map[string]map[string]float64                       `json:"et_star"`
	InterferenceIN       map[string]map[string]map[string]map[string]float64 `json:"interference_i_n"`
	BandwidthBW          map[string]map[string]float64                       `json:"bandwidth_bw"`
	TransferDelay        map[string]map[string]float64                       `json:"transfer_delay"`
	FinancialNetworkCost map[string]map[string]float64                       `json:"financial_network_cost"`
	ContainerOverhead    map[string]map[string]float64                       `json:"container_overhead"`
}

type ScoreBreakdown struct {
	TimeScore         float64 `json:"time_score"`
	CostScore         float64 `json:"cost_score"`
	InterferenceScore float64 `json:"interference_score"`
	TotalScore        float64 `json:"total_score"`
}

type PairwiseInterference struct {
	OtherTaskID string             `json:"other_task_id"`
	Value       float64            `json:"value"`
	Dimensions  map[string]float64 `json:"dimensions"`
}

type CandidateEvaluation struct {
	TaskID                 string                 `json:"task_id"`
	ResourceID             string                 `json:"resource_id"`
	CoreID                 string                 `json:"core_id"`
	Rank                   int                    `json:"rank"`
	Selected               bool                   `json:"selected"`
	StartTime              float64                `json:"start_time"`
	FinishTime             float64                `json:"finish_time"`
	BaseRuntime            float64                `json:"base_runtime"`
	EffectiveRuntime       float64                `json:"effective_runtime"`
	InterferenceTime       float64                `json:"interference_time"`
	TransferDelay          float64                `json:"transfer_delay"`
	BootOverhead           float64                `json:"boot_overhead"`
	ContainerOverhead      float64                `json:"container_overhead"`
	PredecessorFinishFloor float64                `json:"predecessor_finish_floor"`
	RawCost                float64                `json:"raw_cost"`
	PhiN                   float64                `json:"phi_n"`
	PairwiseInterference   []PairwiseInterference `json:"pairwise_interference"`
	Score                  ScoreBreakdown         `json:"score"`
}

type ScheduleStep struct {
	Step               int                   `json:"step"`
	TaskID             string                `json:"task_id"`
	SelectedResourceID string                `json:"selected_resource_id"`
	SelectedCoreID     string                `json:"selected_core_id"`
	SelectedTotalScore float64               `json:"selected_total_score"`
	Candidates         []CandidateEvaluation `json:"candidates"`
}

type Assignment struct {
	TaskID                 string         `json:"task_id"`
	ResourceID             string         `json:"resource_id"`
	CoreID                 string         `json:"core_id"`
	StartTime              float64        `json:"start_time"`
	FinishTime             float64        `json:"finish_time"`
	EffectiveRuntime       float64        `json:"effective_runtime"`
	TransferDelay          float64        `json:"transfer_delay"`
	BootOverhead           float64        `json:"boot_overhead"`
	ContainerOverhead      float64        `json:"container_overhead"`
	PhiN                   float64        `json:"phi_n"`
	Score                  ScoreBreakdown `json:"score"`
	PredecessorFinishFloor float64        `json:"predecessor_finish_floor"`
}

type MachineStopInterval struct {
	ResourceID     string  `json:"resource_id"`
	StopTime       float64 `json:"stop_time"`
	BootStartTime  float64 `json:"boot_start_time"`
	BootFinishTime float64 `json:"boot_finish_time"`
	BootOverhead   float64 `json:"boot_overhead"`
	Reason         string  `json:"reason"`
}

type CostVariables struct {
	CCPU  map[string]float64            `json:"c_cpu"`
	CMem  map[string]float64            `json:"c_mem"`
	CFin  map[string]float64            `json:"c_fin"`
	FC    map[string]float64            `json:"fc"`
	CTN   map[string]map[string]float64 `json:"c_t_n"`
	BUsed float64                       `json:"b_used"`
	PCC   float64                       `json:"p_cc"`
	CW    float64                       `json:"c_w"`
}

type TimingVariables struct {
	ST                      map[string]float64 `json:"st"`
	FT                      map[string]float64 `json:"ft"`
	Avail                   map[string]float64 `json:"avail"`
	Makespan                float64            `json:"makespan"`
	TransferDelayByTask     map[string]float64 `json:"transfer_delay_by_task"`
	BootOverheadByTask      map[string]float64 `json:"boot_overhead_by_task"`
	ContainerOverheadByTask map[string]float64 `json:"container_overhead_by_task"`
}

type InterferenceVariables struct {
	PhiN                  map[string]float64  `json:"phi_n"`
	ETStarByTask          map[string]float64  `json:"et_star_by_task"`
	ColocatedTasks        map[string][]string `json:"colocated_tasks"`
	TotalInterferenceTime float64             `json:"total_interference_time"`
	AveragePhiN           float64             `json:"average_phi_n"`
}

type DeviationVariables struct {
	ETObs   map[string]float64 `json:"et_obs"`
	Var     map[string]float64 `json:"var"`
	Excess  map[string]float64 `json:"excess"`
	DTime   map[string]float64 `json:"d_time"`
	DExcess map[string]float64 `json:"d_excess"`
	DN      map[string]float64 `json:"d_n"`
	DWTime  float64            `json:"d_w_time"`
}

type SchedulerVariables struct {
	XTN      map[string]string  `json:"x_t_n"`
	FTTask   map[string]string  `json:"f_t"`
	SNT      map[string]float64 `json:"s_n_t"`
	AvailN   map[string]float64 `json:"avail_n"`
	ST       map[string]float64 `json:"st"`
	FT       map[string]float64 `json:"ft"`
	Makespan float64            `json:"makespan"`
	BUsed    float64            `json:"b_used"`
}

type GeneratedSimulation struct {
	ID        string     `json:"id"`
	Seed      int64      `json:"seed"`
	Workflow  Workflow   `json:"workflow"`
	Resources []Resource `json:"resources"`
	SLA       SLA        `json:"sla"`
	Matrices  Matrices   `json:"matrices"`
}

type SimulationResult struct {
	ID                    string                `json:"id"`
	Seed                  int64                 `json:"seed"`
	Workflow              Workflow              `json:"workflow"`
	Resources             []Resource            `json:"resources"`
	SLA                   SLA                   `json:"sla"`
	Matrices              Matrices              `json:"matrices"`
	Assignments           []Assignment          `json:"assignments"`
	MachineStopIntervals  []MachineStopInterval `json:"machine_stop_intervals"`
	SchedulerSteps        []ScheduleStep        `json:"scheduler_steps"`
	SchedulerVariables    SchedulerVariables    `json:"scheduler_variables"`
	TimingVariables       TimingVariables       `json:"timing_variables"`
	CostVariables         CostVariables         `json:"cost_variables"`
	InterferenceVariables InterferenceVariables `json:"interference_variables"`
	DeviationVariables    DeviationVariables    `json:"deviation_variables"`
}

type ScheduleConstraints struct {
	BudgetLimit   *float64 `json:"budget_limit"`
	DeadlineLimit *float64 `json:"deadline_limit"`
	OptionCount   int      `json:"option_count"`
	BeamWidth     int      `json:"beam_width"`
}

type ScheduleOption struct {
	ID                  string           `json:"id"`
	Rank                int              `json:"rank"`
	Feasible            bool             `json:"feasible"`
	Recommended         bool             `json:"recommended"`
	BudgetUsed          float64          `json:"budget_used"`
	BudgetLimit         *float64         `json:"budget_limit"`
	BudgetViolation     float64          `json:"budget_violation"`
	Makespan            float64          `json:"makespan"`
	DeadlineLimit       *float64         `json:"deadline_limit"`
	DeadlineViolation   float64          `json:"deadline_violation"`
	MachineSignature    string           `json:"machine_signature"`
	MachineDistribution map[string]int   `json:"machine_distribution"`
	WeightedScore       float64          `json:"weighted_score"`
	WeightedTimePercent float64          `json:"weighted_time_percent"`
	WeightedCostPercent float64          `json:"weighted_cost_percent"`
	DiversityScore      float64          `json:"diversity_score"`
	Result              SimulationResult `json:"result"`
}

type ScheduleOptimizationResponse struct {
	SelectedOptionID *string             `json:"selected_option_id"`
	Constraints      ScheduleConstraints `json:"constraints"`
	Options          []ScheduleOption    `json:"options"`
}
