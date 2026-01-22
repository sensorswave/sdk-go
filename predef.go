package sensorswave

// Predefined constants for events and properties
import (
	"strconv"
)

// Predefined events
const (
	PseAppInstall = "$app_install"
	PseLaunch     = "$launch"
	PseDaily      = "$daily"
	PseIdentify   = "$Identify" // User correlation event
	PseTick       = "$tick"
	PseShow       = "$show"
	PseClick      = "$click"

	PseABImpress = "$ABImpress" // AB impression event

	// Internal events from def package
	PseUserSet = "$UserSet" // User property event
)

// User property operation types
const (
	UserSetTypeSet       = "user_set"       // Set user property
	UserSetTypeSetOnce   = "user_set_once"  // Set once user property
	UserSetTypeIncrement = "user_increment" // Increment user property
	UserSetTypeAppend    = "user_append"    // Append user property
	UserSetTypeUnion     = "user_union"     // Union user property
	UserSetTypeUnset     = "user_unset"     // Unset user property
	UserSetTypeDelete    = "user_delete"    // Delete user property
)

// Predefined property keys
const (
	PspUserSetType = "$user_set_type" // User property set type
)

// Predefined properties
const (
	PspDeviceInfo = "$device_info" // v:string json -- for ad tracking proc & result
	PspIP         = "$ip"          // v:string      -- set by sdk
	PspUA         = "$ua"          // v:string      -- set by sdk
	PspOS         = "$os"          // v:string: ios android harmony -- set by sdk
	PspOSVer      = "$os_version"  // v:string      -- set by sdk
	PspModel      = "$model"       // v:string      -- set by sdk
	PspAppVer     = "$app_version" // v:string      -- set by sdk
	PspSDKType    = "$sdk_type"    // v:string      -- set by sdk
	PspCountry    = "$country"     // v:string      -- set by sdk or geoip
	PspProvince   = "$province"    // v:string      -- set by sdk or geoip
	PspCity       = "$city"        // v:string      -- set by sdk or geoip
)

// FormatABPropertyName returns the AB property name in the unified format "$ab_{ID}".
func FormatABPropertyName(id int) string {
	return "$ab_" + strconv.Itoa(id)
}
