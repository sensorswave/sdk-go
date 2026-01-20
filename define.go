package sensorswave

// SDK information
const (
	SdkName    = "go-sdk"
	SdkVersion = "1.0.0"
)

const (
	HeaderProject         = "Project"
	HeaderSourceToken     = "SourceToken"
	HeaderAccountAPIToken = "AccountApiToken"
)

const (
	maxEventChanSize = 50 * 10         // max 500 events in channel
	maxBatchSize     = 50              // max 50 events in a batch
	maxHTTPBodySize  = 5 * 1024 * 1024 // max 5MB http body in a request
)
