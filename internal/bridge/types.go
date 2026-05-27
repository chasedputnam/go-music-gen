package bridge

// InferenceRequest is the JSON-RPC request sent to the Python worker over stdin.
type InferenceRequest struct {
	ID       string         `json:"id"`
	TaskType string         `json:"task_type"`
	SaveDir  string         `json:"save_dir"`
	Params   InferenceParams `json:"params"`
	Config   InferenceConfig `json:"config"`
}

// InferenceParams holds all ACE-Step GenerationParams fields.
type InferenceParams struct {
	Caption            string   `json:"caption"`
	Lyrics             string   `json:"lyrics"`
	Instrumental       bool     `json:"instrumental"`
	VocalLanguage      string   `json:"vocal_language"`
	Duration           *float64 `json:"duration,omitempty"`
	BPM                *int     `json:"bpm,omitempty"`
	KeyScale           *string  `json:"keyscale,omitempty"`
	TimeSignature      *string  `json:"timesignature,omitempty"`
	InferenceSteps     int      `json:"inference_steps"`
	GuidanceScale      float64  `json:"guidance_scale"`
	Seed               int64    `json:"seed"`
	Thinking           bool     `json:"thinking"`
	// Cover-specific
	ReferenceAudio     string  `json:"reference_audio,omitempty"`
	AudioCoverStrength float64 `json:"audio_cover_strength,omitempty"`
	// Repaint-specific
	SrcAudio        string  `json:"src_audio,omitempty"`
	RepaintingStart float64 `json:"repainting_start,omitempty"`
	RepaintingEnd   float64 `json:"repainting_end,omitempty"`
}

// InferenceConfig holds all ACE-Step GenerationConfig fields.
type InferenceConfig struct {
	BatchSize   int    `json:"batch_size"`
	AudioFormat string `json:"audio_format"`
	Seed        int64  `json:"seed"`
}

// workerLine is the union type read from the Python worker stdout.
// Lines with Type=="progress" are progress events; lines without Type are results.
type workerLine struct {
	ID          string  `json:"id"`
	Type        string  `json:"type,omitempty"`
	Value       float64 `json:"value,omitempty"`
	Stage       string  `json:"stage,omitempty"`
	Success     bool    `json:"success"`
	Error       string  `json:"error,omitempty"`
	LMAvailable bool    `json:"lm_available,omitempty"`
	Device      string  `json:"device,omitempty"`
	Dtype       string  `json:"dtype,omitempty"`
}

// ProgressEvent is sent to callers via the progress channel during inference.
type ProgressEvent struct {
	Value float64
	Stage string
}

// InferenceResponse is the final result returned by Bridge.Generate.
type InferenceResponse struct {
	Success     bool
	Error       string
	LMAvailable bool
	Device      string
	Dtype       string
}
